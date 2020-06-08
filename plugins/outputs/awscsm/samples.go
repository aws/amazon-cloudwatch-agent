package awscsm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"

	"github.com/aws/amazon-cloudwatch-agent/plugins/outputs/awscsm/providers"
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func compressSamples(samples []map[string]interface{}) (string, int64, int64, error) {
	cfg := providers.Config.RetrieveAgentConfig()

	b, err := json.Marshal(samples)
	if err != nil {
		return "", 0, 0, err
	}

	uncompressedLength := len(b)
	if uncompressedLength > cfg.Limits.MaxUncompressedSampleSize {
		return "", 0, 0, fmt.Errorf("uncompressed samples over limit: %d", uncompressedLength)
	}

	checksum := crc32.Checksum(b, crc32cTable)
	buf := &bytes.Buffer{}
	writer := gzip.NewWriter(buf)

	if _, err := writer.Write(b); err != nil {
		return "", 0, 0, err
	}

	// Close will flush everything to the buffer
	writer.Close()
	if buf.Len() > cfg.Limits.MaxCompressedSampleSize {
		return "", 0, 0, fmt.Errorf("compression of samples over limit: %d", buf.Len())
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), int64(checksum), int64(uncompressedLength), nil
}
