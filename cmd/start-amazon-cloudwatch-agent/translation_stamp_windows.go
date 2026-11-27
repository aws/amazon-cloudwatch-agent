// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-cloudwatch-agent/cfg/commonconfig"
	"github.com/aws/amazon-cloudwatch-agent/internal/version"
	"github.com/aws/amazon-cloudwatch-agent/tool/paths"
)

// The translation stamp records everything a successful translation depended
// on. The service start skips the translator only when a freshly computed
// stamp matches the stored one exactly. Any difference, read error or timeout
// means "not current" and the translator runs, as it did before the stamp
// existed.
const (
	translationStampFileName      = "amazon-cloudwatch-agent.translation-stamp.json"
	translationStampSchemaVersion = 1

	// imdsIdentityTimeout bounds how long the stamp check waits for the EC2
	// instance identity (waiting for a network interface, then querying
	// IMDS; a healthy lookup takes well under a second). On timeout the check
	// reports a miss and the translator runs, so a slow or unreachable IMDS
	// delays startup by at most this value compared with always translating.
	imdsIdentityTimeout = 5 * time.Second

	// The following mirror translator/util/ec2util and the IMDS client it
	// uses (override/aws in opentelemetry-collector-contrib), which this
	// binary does not import to stay small: up to 5 x 1 s wait for a network
	// interface, IMDS_NUMBER_RETRY retries (default 1), IMDSv2 first, then
	// IMDSv1 fallback unless AWS_EC2_METADATA_V1_DISABLED is true.
	networkWaitRetries       = 5
	imdsRetriesEnvVar        = "IMDS_NUMBER_RETRY"
	defaultIMDSRetries       = 1
	imdsV1DisabledEnvVar     = "AWS_EC2_METADATA_V1_DISABLED"
	imdsHostnamePath         = "hostname"
	networkWaitRetryInterval = time.Second

	// dateLayout and datePlaceholder mirror ResolvePlaceholder in
	// translator/translate/util/placeholderUtil.go, which resolves {date}
	// at translation time with the local time zone.
	dateLayout      = "2006-01-02"
	datePlaceholder = "{date}"

	// asgPlaceholder is resolved at translation time through the EC2
	// DescribeTags API, which needs credentials and network access. Configs
	// that use it are always translated.
	asgPlaceholder = "${aws:AutoScalingGroupName}"

	unknownAgentVersion = "Unknown"
)

// unresolvedPlaceholderValues are the fallback values the translator writes
// when instance metadata was not available during translation. A stamp is
// never written for outputs that contain one of them, so a translation done
// without IMDS is not reused.
var unresolvedPlaceholderValues = []string{
	"i-UNKNOWN",
	"UNKNOWN-HOST",
	"UNKNOWN-IP",
	"UNKNOWN-REGION",
	"UNKNOWN-ACCOUNT",
	"UNKNOWN-TYPE",
	"UNKNOWN-AMI",
}

// Injection points for tests.
var (
	agentVersion       = version.Number
	lookupHostIdentity = func() (hostIdentity, error) {
		return fetchHostIdentity(func(ctx context.Context) instanceMetadata {
			waitForNetwork(ctx)
			return imdsInstanceMetadata(ctx)
		}, imdsIdentityTimeout)
	}
	localHostname          = os.Hostname
	now                    = time.Now
	defaultSharedConfigDir = func() string {
		// Same location the translator uses on Windows (detectHomeDirectory in
		// translator/util/sdkutil.go): the agent runs as SYSTEM.
		systemDrive := os.Getenv("SystemDrive")
		if systemDrive == "" {
			systemDrive = "C:"
		}
		return systemDrive + `\Users\Administrator\.aws`
	}
)

// The host identity is looked up at most once per process, like the ec2util
// singleton the translator uses.
var (
	hostIdentityLookedUp  bool
	cachedHostIdentity    hostIdentity
	cachedHostIdentityErr error
)

type binaryInfo struct {
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	ModTimeUnixNano int64  `json:"mod_time_unix_nano"`
}

// hostIdentity holds the raw sources the translator resolves placeholders,
// region and mode from.
type hostIdentity struct {
	InstanceID    string `json:"instance_id"`
	IMDSHostname  string `json:"imds_hostname"`
	LocalHostname string `json:"local_hostname"`
	PrivateIP     string `json:"private_ip"`
	Region        string `json:"region"`
	AccountID     string `json:"account_id"`
	ImageID       string `json:"image_id"`
	InstanceType  string `json:"instance_type"`
}

type translationStamp struct {
	SchemaVersion int          `json:"schema_version"`
	AgentVersion  string       `json:"agent_version"`
	Binaries      []binaryInfo `json:"binaries"`
	InputsSHA256  string       `json:"inputs_sha256"`
	OutputsSHA256 string       `json:"outputs_sha256"`
	// EnvironmentSHA256 covers the whole process environment, which the
	// translator inherits. Only the hash is stored, never the values.
	EnvironmentSHA256 string       `json:"environment_sha256"`
	Identity          hostIdentity `json:"identity"`
	// Date is set only when an input contains {date}.
	Date string `json:"date"`
}

// inputStamp holds the parts of the stamp the translator depends on that are
// known before it runs: agent version, binaries, the input hash, the
// environment hash and the {date} value. It is captured before translateConfig
// and compared again afterwards, so a config or environment change during a
// long translation is never recorded as current.
type inputStamp struct {
	agentVersion      string
	binaries          []binaryInfo
	inputsSHA256      string
	environmentSHA256 string
	date              string
}

func (s inputStamp) equal(other inputStamp) bool {
	return s.agentVersion == other.agentVersion &&
		slices.Equal(s.binaries, other.binaries) &&
		s.inputsSHA256 == other.inputsSHA256 &&
		s.environmentSHA256 == other.environmentSHA256 &&
		s.date == other.date
}

func (s inputStamp) toStamp(outputsSHA256 string) translationStamp {
	return translationStamp{
		SchemaVersion:     translationStampSchemaVersion,
		AgentVersion:      s.agentVersion,
		Binaries:          s.binaries,
		InputsSHA256:      s.inputsSHA256,
		OutputsSHA256:     outputsSHA256,
		EnvironmentSHA256: s.environmentSHA256,
		Date:              s.date,
	}
}

// translationInputs carries the pre-translation state from prepareConfig to
// writeTranslationStamp: the input stamp captured before the translator ran
// and the host identity looked up in parallel with it.
type translationInputs struct {
	stamp    inputStamp
	stampErr error
	identity <-chan identityResult
}

type identityResult struct {
	identity hostIdentity
	err      error
}

func translationStampPath() string {
	return filepath.Join(filepath.Dir(paths.TomlConfigPath), translationStampFileName)
}

// translationIsCurrent reports whether the existing TOML, YAML and
// env-config.json can be used without running the translator. It logs the
// category of the first difference it finds.
func translationIsCurrent() bool {
	if reason := translationMissReason(); reason != "" {
		log.Printf("I! Config translation required: %s\n", reason)
		return false
	}
	return true
}

func translationMissReason() string {
	stored, err := readTranslationStamp()
	if errors.Is(err, fs.ErrNotExist) {
		return "no translation stamp from a previous successful translation"
	}
	if err != nil {
		return "translation stamp unreadable"
	}
	if stored.SchemaVersion != translationStampSchemaVersion {
		return "translation stamp schema version changed"
	}

	// Compare everything that is available locally before querying IMDS.
	current, err := computeCurrentStamp()
	if err != nil {
		return err.Error()
	}
	if reason := compareLocalStamp(stored, current); reason != "" {
		return reason
	}

	identity, err := currentHostIdentity()
	if err != nil {
		return fmt.Sprintf("host identity unavailable (%v)", err)
	}
	if identity != stored.Identity {
		return "host identity changed"
	}
	return ""
}

func compareLocalStamp(stored, current translationStamp) string {
	switch {
	case stored.AgentVersion != current.AgentVersion:
		return "agent version changed"
	case !slices.Equal(stored.Binaries, current.Binaries):
		return "agent binaries changed"
	case stored.InputsSHA256 != current.InputsSHA256:
		return "config inputs changed"
	case stored.OutputsSHA256 != current.OutputsSHA256:
		return "translated outputs changed"
	case stored.EnvironmentSHA256 != current.EnvironmentSHA256:
		return "environment changed"
	case stored.Date != current.Date:
		return "date changed ({date} placeholder)"
	}
	return ""
}

// captureTranslationInputs records the state the translator will depend on
// before it runs and starts the host identity lookup in parallel, so the IMDS
// round trip overlaps the translator instead of adding to startup time. It is
// called before translateConfig.
func captureTranslationInputs() translationInputs {
	identity := make(chan identityResult, 1)
	lookup := lookupHostIdentity
	go func() {
		id, err := lookup()
		identity <- identityResult{identity: id, err: err}
	}()
	stamp, err := computeInputStamp()
	return translationInputs{stamp: stamp, stampErr: err, identity: identity}
}

// writeTranslationStamp records the state after a successful translation, using
// the inputs captured before it ran. Failing to write it is not an error: the
// next start translates again.
func writeTranslationStamp(captured translationInputs) {
	stamp, err := stampForSuccessfulTranslation(captured)
	if err != nil {
		log.Printf("I! Not caching the config translation: %v\n", err)
		return
	}
	data, err := json.MarshalIndent(stamp, "", "  ")
	if err != nil {
		log.Printf("W! Cannot encode translation stamp: %v\n", err)
		return
	}
	if err = writeFileAtomic(translationStampPath(), data); err != nil {
		log.Printf("W! Cannot write translation stamp %s: %v\n", translationStampPath(), err)
	}
}

// removeTranslationStamp invalidates the stamp before the translator runs, so
// a failed or interrupted translation never leaves a valid stamp behind.
func removeTranslationStamp() {
	if err := os.Remove(translationStampPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("W! Cannot remove translation stamp %s: %v\n", translationStampPath(), err)
	}
}

func stampForSuccessfulTranslation(captured translationInputs) (translationStamp, error) {
	if captured.stampErr != nil {
		return translationStamp{}, captured.stampErr
	}
	// Recompute the input side and refuse to cache if a config file, the
	// environment or the date changed while the translator ran. The outputs
	// would then reflect inputs that no longer match the stamp.
	after, err := computeInputStamp()
	if err != nil {
		return translationStamp{}, err
	}
	if !captured.stamp.equal(after) {
		return translationStamp{}, errors.New("config inputs changed during translation")
	}
	outputsSum, outputs, err := hashFiles([]string{paths.TomlConfigPath, paths.YamlConfigPath, paths.EnvConfigPath})
	if err != nil {
		return translationStamp{}, fmt.Errorf("translated outputs unreadable (%w)", err)
	}
	if _, err = os.Stat(paths.TomlConfigPath); err != nil {
		return translationStamp{}, errors.New("TOML config missing")
	}
	for _, content := range outputs {
		for _, value := range unresolvedPlaceholderValues {
			if strings.Contains(string(content), value) {
				return translationStamp{}, fmt.Errorf("translated output contains %q", value)
			}
		}
	}
	result := <-captured.identity
	if result.err != nil {
		return translationStamp{}, fmt.Errorf("host identity unavailable (%w)", result.err)
	}
	stamp := captured.stamp.toStamp(outputsSum)
	stamp.Identity = result.identity
	return stamp, nil
}

func readTranslationStamp() (translationStamp, error) {
	var stamp translationStamp
	data, err := os.ReadFile(filepath.Clean(translationStampPath()))
	if err != nil {
		return stamp, err
	}
	if err = json.Unmarshal(data, &stamp); err != nil {
		return stamp, err
	}
	return stamp, nil
}

// computeInputStamp hashes everything the translator depends on except its own
// outputs: agent version, binaries, inputs, environment and the {date} value.
func computeInputStamp() (inputStamp, error) {
	agentVer := agentVersion()
	if agentVer == "" || agentVer == unknownAgentVersion {
		return inputStamp{}, errors.New("agent version unknown")
	}
	binaries, err := binaryFingerprints()
	if err != nil {
		return inputStamp{}, fmt.Errorf("agent binaries unreadable (%w)", err)
	}

	inputFiles, err := translationInputFiles()
	if err != nil {
		return inputStamp{}, fmt.Errorf("config inputs unreadable (%w)", err)
	}
	inputsSum, inputs, err := hashFiles(inputFiles)
	if err != nil {
		return inputStamp{}, fmt.Errorf("config inputs unreadable (%w)", err)
	}
	usesDate := false
	for _, content := range inputs {
		if strings.Contains(string(content), asgPlaceholder) {
			return inputStamp{}, fmt.Errorf("config uses %s, which is always resolved by the translator", asgPlaceholder)
		}
		if strings.Contains(string(content), datePlaceholder) {
			usesDate = true
		}
	}

	stamp := inputStamp{
		agentVersion:      agentVer,
		binaries:          binaries,
		inputsSHA256:      inputsSum,
		environmentSHA256: environmentSHA256(),
	}
	if usesDate {
		stamp.date = now().Format(dateLayout)
	}
	return stamp, nil
}

// computeCurrentStamp builds the full stamp for the current on-disk state,
// including the output hash. It is used by the translation check.
func computeCurrentStamp() (translationStamp, error) {
	inputs, err := computeInputStamp()
	if err != nil {
		return translationStamp{}, err
	}
	outputsSum, _, err := hashFiles([]string{paths.TomlConfigPath, paths.YamlConfigPath, paths.EnvConfigPath})
	if err != nil {
		return translationStamp{}, fmt.Errorf("translated outputs unreadable (%w)", err)
	}
	if _, err = os.Stat(paths.TomlConfigPath); err != nil {
		return translationStamp{}, errors.New("TOML config missing")
	}
	return inputs.toStamp(outputsSum), nil
}

// binaryFingerprints returns size and modification time of the agent, the
// translator and this binary. They are not hashed: reading tens of MB during
// a loaded boot would cost more than the check is meant to save.
func binaryFingerprints() ([]binaryInfo, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	binaries := make([]binaryInfo, 0, 3)
	for _, p := range []string{paths.AgentBinaryPath, paths.TranslatorBinaryPath, self} {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		binaries = append(binaries, binaryInfo{Path: p, Size: info.Size(), ModTimeUnixNano: info.ModTime().UnixNano()})
	}
	return binaries, nil
}

// translationInputFiles lists every file the translator may read on a service
// start: the JSON config, every file under Configs (recursively, including
// .tmp and .yaml), common-config.toml and the shared credentials/config files
// the region lookup may read. Paths that do not exist are part of the list,
// so creating them later changes the hash.
func translationInputFiles() ([]string, error) {
	files := []string{paths.JsonConfigPath, paths.CommonConfigPath}

	err := filepath.WalkDir(paths.ConfigDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == paths.ConfigDirPath && errors.Is(err, fs.ErrNotExist) {
				files = append(files, path)
				return nil
			}
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	credentialFiles, err := sharedConfigFiles()
	if err != nil {
		return nil, err
	}
	files = append(files, credentialFiles...)

	sort.Strings(files)
	return slices.Compact(files), nil
}

// sharedConfigFiles mirrors the files SDKRegionWithCredsMap
// (translator/util/sdkutil.go) and SharedConfigRegion (cfg/aws/region.go)
// may read: the shared_credential_file from common-config.toml and the
// "config" file next to it, the AWS_SHARED_CREDENTIALS_FILE/AWS_CONFIG_FILE
// overrides, and the default files in the home directory the translator sets.
func sharedConfigFiles() ([]string, error) {
	dir := defaultSharedConfigDir()
	files := []string{filepath.Join(dir, "credentials"), filepath.Join(dir, "config")}
	for _, name := range []string{"AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE"} {
		if value := os.Getenv(name); value != "" {
			files = append(files, value)
		}
	}

	data, err := os.ReadFile(filepath.Clean(paths.CommonConfigPath))
	if errors.Is(err, fs.ErrNotExist) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}
	conf, err := commonconfig.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	if credentialFile, ok := conf.CredentialsMap()[commonconfig.CredentialFile]; ok && credentialFile != "" {
		files = append(files, credentialFile, filepath.Join(filepath.Dir(credentialFile), "config"))
	}
	return files, nil
}

// hashFiles returns a SHA-256 over path, presence, length and content of each
// file, in the given order, plus the contents of the files that exist.
func hashFiles(files []string) (string, [][]byte, error) {
	h := sha256.New()
	contents := make([][]byte, 0, len(files))
	for _, p := range files {
		data, err := os.ReadFile(filepath.Clean(p))
		if errors.Is(err, fs.ErrNotExist) {
			writeHashFields(h, p, "absent")
			continue
		}
		if err != nil {
			return "", nil, err
		}
		writeHashFields(h, p, "present", strconv.Itoa(len(data)))
		h.Write(data)
		contents = append(contents, data)
	}
	return hex.EncodeToString(h.Sum(nil)), contents, nil
}

func writeHashFields(h hash.Hash, fields ...string) {
	for _, field := range fields {
		h.Write([]byte(field))
		h.Write([]byte{0})
	}
}

func environmentSHA256() string {
	env := os.Environ()
	sort.Strings(env)
	h := sha256.New()
	writeHashFields(h, env...)
	return hex.EncodeToString(h.Sum(nil))
}

func currentHostIdentity() (hostIdentity, error) {
	if !hostIdentityLookedUp {
		cachedHostIdentity, cachedHostIdentityErr = lookupHostIdentity()
		hostIdentityLookedUp = true
	}
	return cachedHostIdentity, cachedHostIdentityErr
}

// instanceMetadata holds the fields translator/util/ec2util.EC2Util derives
// from IMDS (deriveEC2MetadataFromIMDS).
type instanceMetadata struct {
	Region       string
	PrivateIP    string
	InstanceID   string
	Hostname     string
	AccountID    string
	InstanceType string
	ImageID      string
}

// waitForNetwork mirrors initEC2UtilSingleton in translator/util/ec2util:
// it waits up to networkWaitRetries x 1 s for an up, non-loopback interface.
func waitForNetwork(ctx context.Context) {
	for attempt := 0; attempt < networkWaitRetries; attempt++ {
		ifs, _ := net.Interfaces()
		for _, in := range ifs {
			if in.Flags&net.FlagUp != 0 && in.Flags&net.FlagLoopback == 0 {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(networkWaitRetryInterval):
		}
	}
}

// imdsInstanceMetadata mirrors deriveEC2MetadataFromIMDS in
// translator/util/ec2util: the hostname from the "hostname" metadata path and
// the rest from the instance identity document. A failed request leaves its
// fields empty.
func imdsInstanceMetadata(ctx context.Context, optFns ...func(*imds.Options)) instanceMetadata {
	var md instanceMetadata
	strict, permissive := newIMDSClients(optFns...)

	hostname, err := withIMDSFallback(strict, permissive, func(c *imds.Client) (string, error) {
		out, err := c.GetMetadata(ctx, &imds.GetMetadataInput{Path: imdsHostnamePath})
		if err != nil {
			return "", err
		}
		defer out.Content.Close()
		content, err := io.ReadAll(out.Content)
		return string(content), err
	})
	if err == nil {
		md.Hostname = hostname
	}

	doc, err := withIMDSFallback(strict, permissive, func(c *imds.Client) (imds.InstanceIdentityDocument, error) {
		out, err := c.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
		if err != nil {
			return imds.InstanceIdentityDocument{}, err
		}
		return out.InstanceIdentityDocument, nil
	})
	if err == nil {
		md.Region = doc.Region
		md.AccountID = doc.AccountID
		md.PrivateIP = doc.PrivateIP
		md.InstanceID = doc.InstanceID
		md.InstanceType = doc.InstanceType
		md.ImageID = doc.ImageID
	}
	return md
}

// newIMDSClients mirrors override/aws NewIMDSClient: a strict IMDSv2-only
// client with retries, and a permissive client that allows the SDK's IMDSv1
// fallback unless AWS_EC2_METADATA_V1_DISABLED is true.
func newIMDSClients(optFns ...func(*imds.Options)) (*imds.Client, *imds.Client) {
	retries := imdsRetries()
	strictOpts := append(slices.Clone(optFns), func(o *imds.Options) {
		o.Retryer = newIMDSRetryer(retries)
		o.EnableFallback = aws.FalseTernary
	})
	permissiveOpts := append(slices.Clone(optFns), func(o *imds.Options) {
		if strings.EqualFold(os.Getenv(imdsV1DisabledEnvVar), "true") {
			o.EnableFallback = aws.FalseTernary
		}
	})
	return imds.New(imds.Options{}, strictOpts...), imds.New(imds.Options{}, permissiveOpts...)
}

func withIMDSFallback[T any](strict, permissive *imds.Client, fn func(*imds.Client) (T, error)) (T, error) {
	if result, err := fn(strict); err == nil {
		return result, nil
	}
	return fn(permissive)
}

// imdsRetries mirrors override/aws GetDefaultRetryNumber.
func imdsRetries() int {
	if v := os.Getenv(imdsRetriesEnvVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultIMDSRetries
}

// newIMDSRetryer mirrors override/aws IMDSRetryer: the SDK standard retryer
// that also retries any *smithyhttp.ResponseError.
func newIMDSRetryer(retries int) aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) {
		o.MaxAttempts = retries + 1
		o.Retryables = append([]retry.IsErrorRetryable{retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
			var responseErr *smithyhttp.ResponseError
			if errors.As(err, &responseErr) {
				return aws.TrueTernary
			}
			return aws.UnknownTernary
		})}, o.Retryables...)
	})
}

// fetchHostIdentity runs fetch with a deadline of timeout. The translator is a
// separate process that queries IMDS on its own.
func fetchHostIdentity(fetch func(context.Context) instanceMetadata, timeout time.Duration) (hostIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result := make(chan instanceMetadata, 1)
	go func() {
		result <- fetch(ctx)
	}()

	var md instanceMetadata
	select {
	case md = <-result:
	case <-ctx.Done():
		return hostIdentity{}, fmt.Errorf("EC2 instance metadata not available within %s", timeout)
	}

	// A partial identity means IMDS did not answer every request, so the
	// translator may have fallen back to other values. Do not cache then.
	if md.InstanceID == "" || md.Hostname == "" || md.PrivateIP == "" || md.Region == "" ||
		md.AccountID == "" || md.ImageID == "" || md.InstanceType == "" {
		return hostIdentity{}, errors.New("EC2 instance metadata incomplete")
	}

	hostname, err := localHostname()
	if err != nil {
		return hostIdentity{}, fmt.Errorf("local hostname: %w", err)
	}

	return hostIdentity{
		InstanceID:    md.InstanceID,
		IMDSHostname:  md.Hostname,
		LocalHostname: hostname,
		PrivateIP:     md.PrivateIP,
		Region:        md.Region,
		AccountID:     md.AccountID,
		ImageID:       md.ImageID,
		InstanceType:  md.InstanceType,
	}, nil
}

// writeFileAtomic writes data to a temporary file in the target directory and
// renames it over path. On Windows os.Rename replaces an existing file
// (MoveFileEx with MOVEFILE_REPLACE_EXISTING).
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "translation-stamp-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
