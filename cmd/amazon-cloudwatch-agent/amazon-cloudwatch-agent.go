// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // Comment this line to disable pprof endpoint.
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	configaws "github.com/aws/amazon-cloudwatch-agent/cfg/aws"
	"github.com/aws/amazon-cloudwatch-agent/cfg/envconfig"
	"github.com/influxdata/wlog"

	"github.com/aws/amazon-cloudwatch-agent/cfg/agentinfo"
	"github.com/aws/amazon-cloudwatch-agent/cfg/migrate"
	"github.com/aws/amazon-cloudwatch-agent/cmd/amazon-cloudwatch-agent/internal"
	"github.com/aws/amazon-cloudwatch-agent/logs"
	_ "github.com/aws/amazon-cloudwatch-agent/plugins"
	"github.com/aws/amazon-cloudwatch-agent/profiler"

	"github.com/influxdata/telegraf/agent"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/logger"

	//_ "github.com/influxdata/telegraf/plugins/aggregators/all"
	"github.com/influxdata/telegraf/plugins/inputs"
	//_ "github.com/influxdata/telegraf/plugins/inputs/all"
	"github.com/influxdata/telegraf/plugins/outputs"

	"github.com/kardianos/service"
)

const (
	defaultEnvCfgFileName = "env-config.json"
	LogTargetEventLog     = "eventlog"
)

var fDebug = flag.Bool("debug", false,
	"turn on debug logging")
var pprofAddr = flag.String("pprof-addr", "",
	"pprof address to listen on, not activate pprof if empty")
var fQuiet = flag.Bool("quiet", false,
	"run in quiet mode")
var fTest = flag.Bool("test", false, "enable test mode: gather metrics, print them out, and exit")
var fTestWait = flag.Int("test-wait", 0, "wait up to this many seconds for service inputs to complete in test mode")
var fSchemaTest = flag.Bool("schematest", false, "validate the toml file schema")
var fConfig = flag.String("config", "", "configuration file to load")
var fEnvConfig = flag.String("envconfig", "", "env configuration file to load")
var fConfigDirectory = flag.String("config-directory", "",
	"directory containing additional *.conf files")
var fVersion = flag.Bool("version", false, "display the version and exit")
var fSampleConfig = flag.Bool("sample-config", false,
	"print out full sample configuration")
var fPidfile = flag.String("pidfile", "", "file to write our pid to")
var fSectionFilters = flag.String("section-filter", "",
	"filter the sections to print, separator is ':'. Valid values are 'agent', 'global_tags', 'outputs', 'processors', 'aggregators' and 'inputs'")
var fInputFilters = flag.String("input-filter", "",
	"filter the inputs to enable, separator is :")
var fInputList = flag.Bool("input-list", false,
	"print available input plugins.")
var fOutputFilters = flag.String("output-filter", "",
	"filter the outputs to enable, separator is :")
var fOutputList = flag.Bool("output-list", false,
	"print available output plugins.")
var fAggregatorFilters = flag.String("aggregator-filter", "",
	"filter the aggregators to enable, separator is :")
var fProcessorFilters = flag.String("processor-filter", "",
	"filter the processors to enable, separator is :")
var fUsage = flag.String("usage", "",
	"print usage for a plugin, ie, 'telegraf --usage mysql'")
var fService = flag.String("service", "",
	"operate on the service (windows only)")
var fServiceName = flag.String("service-name", "telegraf", "service name (windows only)")
var fServiceDisplayName = flag.String("service-display-name", "Telegraf Data Collector Service", "service display name (windows only)")
var fRunAsConsole = flag.Bool("console", false, "run as console application (windows only)")
var fSetEnv = flag.String("setenv", "", "set an env in the configuration file in the format of KEY=VALUE")

var (
	version string
	commit  string
	branch  string
)

var stop chan struct{}

func reloadLoop(
	stop chan struct{},
	inputFilters []string,
	outputFilters []string,
	aggregatorFilters []string,
	processorFilters []string,
) {
	reload := make(chan bool, 1)
	reload <- true
	for <-reload {
		reload <- false

		ctx, cancel := context.WithCancel(context.Background())

		signals := make(chan os.Signal)
		signal.Notify(signals, os.Interrupt, syscall.SIGHUP,
			syscall.SIGTERM, syscall.SIGINT)
		go func() {
			select {
			case sig := <-signals:
				if sig == syscall.SIGHUP {
					log.Printf("I! Reloading Telegraf config")
					<-reload
					reload <- true
				}
				cancel()
			case <-stop:
				cancel()
			}
		}()

		go func(ctx context.Context) {
			profilerTicker := time.NewTicker(60 * time.Second)
			defer profilerTicker.Stop()
			for {
				select {
				case <-profilerTicker.C:
					profiler.Profiler.ReportAndClear()
				case <-ctx.Done():
					profiler.Profiler.ReportAndClear()
					log.Printf("I! Profiler is stopped during shutdown\n")
					return
				}
			}
		}(ctx)

		if envConfigPath, err := getEnvConfigPath(*fConfig, *fEnvConfig); err == nil {
			// Reloads environment variables when file is changed
			go func(ctx context.Context, envConfigPath string) {
				var previousModTime time.Time
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if info, err := os.Stat(envConfigPath); err == nil && info.ModTime().After(previousModTime) {
							if err := loadEnvironmentVariables(envConfigPath); err != nil {
								log.Printf("E! Unable to load env variables: %v", err)
							}
							// Sets the log level based on environment variable
							logLevel := os.Getenv(envconfig.CWAGENT_LOG_LEVEL)
							if logLevel == "" {
								logLevel = "INFO"
							}
							if err := wlog.SetLevelFromName(logLevel); err != nil {
								log.Printf("E! Unable to set log level: %v", err)
							}
							// Set AWS SDK logging
							sdkLogLevel := os.Getenv(envconfig.AWS_SDK_LOG_LEVEL)
							configaws.SetSDKLogLevel(sdkLogLevel)
							previousModTime = info.ModTime()
						}
					case <-ctx.Done():
						return
					}
				}
			}(ctx, envConfigPath)
		}

		err := runAgent(ctx, inputFilters, outputFilters)
		if err != nil && err != context.Canceled {
			log.Fatalf("E! [telegraf] Error running agent: %v", err)
		}
	}
}

// loadEnvironmentVariables updates OS ENV vars with key/val from the given JSON file.
// The "config-translator" program populates that file.
func loadEnvironmentVariables(path string) error {
	if path == "" {
		return fmt.Errorf("No env config file specified")
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Can't read env config file %s due to: %s", path, err.Error())
	}
	envVars := map[string]string{}
	err = json.Unmarshal(bytes, &envVars)
	if err != nil {
		return fmt.Errorf("Can't create env config due to: %s", err.Error())
	}

	for key, val := range envVars {
		os.Setenv(key, val)
		log.Printf("I! %s is set to \"%s\"", key, val)
	}
	return nil
}

func getEnvConfigPath(configPath, envConfigPath string) (string, error) {
	if configPath == "" {
		return "", fmt.Errorf("No config file specified")
	}
	//load the environment variables that's saved in json env config file
	if envConfigPath == "" {
		dir, _ := filepath.Split(configPath)
		envConfigPath = filepath.Join(dir, defaultEnvCfgFileName)
	}
	return envConfigPath, nil
}

func runAgent(ctx context.Context,
	inputFilters []string,
	outputFilters []string,
) error {
	envConfigPath, err := getEnvConfigPath(*fConfig, *fEnvConfig)
	if err != nil {
		return err
	}
	err = loadEnvironmentVariables(envConfigPath)
	if err != nil && !*fSchemaTest {
		log.Printf("W! Failed to load environment variables due to %s", err.Error())
	}
	// If no other options are specified, load the config file and run.
	c := config.NewConfig()
	c.OutputFilters = outputFilters
	c.InputFilters = inputFilters

	err = loadTomlConfigIntoAgent(c)

	if err != nil {
		return err
	}

	err = validateAgentFinalConfigAndPlugins(c)

	if err != nil {
		return err
	}

	ag, err := agent.NewAgent(c)
	if err != nil {
		return err
	}

	// Setup logging as configured.
	logConfig := logger.LogConfig{
		Debug:               ag.Config.Agent.Debug || *fDebug,
		Quiet:               ag.Config.Agent.Quiet || *fQuiet,
		LogTarget:           ag.Config.Agent.LogTarget,
		Logfile:             ag.Config.Agent.Logfile,
		RotationInterval:    ag.Config.Agent.LogfileRotationInterval,
		RotationMaxSize:     ag.Config.Agent.LogfileRotationMaxSize,
		RotationMaxArchives: ag.Config.Agent.LogfileRotationMaxArchives,
		LogWithTimezone:     "",
	}

	logger.SetupLogging(logConfig)
	log.Printf("I! Starting AmazonCloudWatchAgent %s\n", agentinfo.FullVersion())
	// Need to set SDK log level before plugins get loaded.
	// Some aws.Config objects get created early and live forever which means
	// we cannot change the sdk log level without restarting the Agent.
	// For example CloudWatch.Connect().
	sdkLogLevel := os.Getenv(envconfig.AWS_SDK_LOG_LEVEL)
	configaws.SetSDKLogLevel(sdkLogLevel)
	if sdkLogLevel == "" {
		log.Printf("I! AWS SDK log level not set")
	} else {
		log.Printf("I! AWS SDK log level, %s", sdkLogLevel)
	}

	if *fTest || *fTestWait != 0 {
		testWaitDuration := time.Duration(*fTestWait) * time.Second
		return ag.Test(ctx, testWaitDuration)
	}

	log.Printf("I! Loaded inputs: %s", strings.Join(c.InputNames(), " "))
	log.Printf("I! Loaded aggregators: %s", strings.Join(c.AggregatorNames(), " "))
	log.Printf("I! Loaded processors: %s", strings.Join(c.ProcessorNames(), " "))
	log.Printf("I! Loaded outputs: %s", strings.Join(c.OutputNames(), " "))
	log.Printf("I! Tags enabled: %s", c.ListTags())

	agentinfo.SetComponents(c)

	if *fPidfile != "" {
		f, err := os.OpenFile(*fPidfile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("E! Unable to create pidfile: %s", err)
		} else {
			fmt.Fprintf(f, "%d\n", os.Getpid())

			f.Close()

			defer func() {
				err := os.Remove(*fPidfile)
				if err != nil {
					log.Printf("E! Unable to remove pidfile: %s", err)
				}
			}()
		}
	}
	logAgent := logs.NewLogAgent(c)
	go logAgent.Run(ctx)
	return ag.Run(ctx)
}

type program struct {
	inputFilters      []string
	outputFilters     []string
	aggregatorFilters []string
	processorFilters  []string
}

func (p *program) Start(_ service.Service) error {
	go p.run()
	return nil
}
func (p *program) run() {
	stop = make(chan struct{})
	reloadLoop(
		stop,
		p.inputFilters,
		p.outputFilters,
		p.aggregatorFilters,
		p.processorFilters,
	)
}
func (p *program) Stop(_ service.Service) error {
	close(stop)
	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()

	sectionFilters, inputFilters, outputFilters := []string{}, []string{}, []string{}
	if *fSectionFilters != "" {
		sectionFilters = strings.Split(":"+strings.TrimSpace(*fSectionFilters)+":", ":")
	}
	if *fInputFilters != "" {
		inputFilters = strings.Split(":"+strings.TrimSpace(*fInputFilters)+":", ":")
	}
	if *fOutputFilters != "" {
		outputFilters = strings.Split(":"+strings.TrimSpace(*fOutputFilters)+":", ":")
	}

	aggregatorFilters, processorFilters := []string{}, []string{}
	if *fAggregatorFilters != "" {
		aggregatorFilters = strings.Split(":"+strings.TrimSpace(*fAggregatorFilters)+":", ":")
	}
	if *fProcessorFilters != "" {
		processorFilters = strings.Split(":"+strings.TrimSpace(*fProcessorFilters)+":", ":")
	}

	logger.SetupLogging(logger.LogConfig{})

	if *pprofAddr != "" {
		go func() {
			pprofHostPort := *pprofAddr
			parts := strings.Split(pprofHostPort, ":")
			if len(parts) == 2 && parts[0] == "" {
				pprofHostPort = fmt.Sprintf("localhost:%s", parts[1])
			}
			pprofHostPort = "http://" + pprofHostPort + "/debug/pprof"

			log.Printf("I! Starting pprof HTTP server at: %s", pprofHostPort)

			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				log.Fatal("E! " + err.Error())
			}
		}()
	}

	if len(args) > 0 {
		switch args[0] {
		case "version":
			fmt.Println(agentinfo.FullVersion())
			return
		case "config":
			config.PrintSampleConfig(
				sectionFilters,
				inputFilters,
				outputFilters,
				aggregatorFilters,
				processorFilters,
			)
			return
		}
	}

	// switch for flags which just do something and exit immediately
	switch {
	case *fOutputList:
		fmt.Println("Available Output Plugins: ")
		names := make([]string, 0, len(outputs.Outputs))
		for k := range outputs.Outputs {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			fmt.Printf("  %s\n", k)
		}
		return
	case *fInputList:
		fmt.Println("Available Input Plugins:")
		names := make([]string, 0, len(inputs.Inputs))
		for k := range inputs.Inputs {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			fmt.Printf("  %s\n", k)
		}
		return
	case *fVersion:
		fmt.Println(agentinfo.FullVersion())
		return
	case *fSampleConfig:
		config.PrintSampleConfig(
			sectionFilters,
			inputFilters,
			outputFilters,
			aggregatorFilters,
			processorFilters,
		)
		return
	case *fUsage != "":
		err := config.PrintInputConfig(*fUsage)
		err2 := config.PrintOutputConfig(*fUsage)
		if err != nil && err2 != nil {
			log.Fatalf("E! %s and %s", err, err2)
		}
		return
	case *fSetEnv != "":
		if *fEnvConfig != "" {
			parts := strings.SplitN(*fSetEnv, "=", 2)
			if len(parts) == 2 {
				bytes, err := os.ReadFile(*fEnvConfig)
				if err != nil {
					log.Fatalf("E! Failed to read env config: %v", err)
				}
				envVars := map[string]string{}
				err = json.Unmarshal(bytes, &envVars)
				if err != nil {
					log.Fatalf("E! Failed to unmarshal env config: %v", err)
				}
				envVars[parts[0]] = parts[1]
				bytes, err = json.MarshalIndent(envVars, "", "\t")
				if err = os.WriteFile(*fEnvConfig, bytes, 0644); err != nil {
					log.Fatalf("E! Failed to update env config: %v", err)
				}
			}
		}
		return
	}

	if runtime.GOOS == "windows" && windowsRunAsService() {
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" { // Should never happen
			programFiles = "C:\\Program Files"
		}
		svcConfig := &service.Config{
			Name:        *fServiceName,
			DisplayName: *fServiceDisplayName,
			Description: "Collects data using a series of plugins and publishes it to" +
				"another series of plugins.",
			Arguments: []string{"--config", programFiles + "\\Telegraf\\telegraf.conf"},
		}

		prg := &program{
			inputFilters:      inputFilters,
			outputFilters:     outputFilters,
			aggregatorFilters: aggregatorFilters,
			processorFilters:  processorFilters,
		}
		s, err := service.New(prg, svcConfig)
		if err != nil {
			log.Fatal("E! " + err.Error())
		}
		// Handle the --service flag here to prevent any issues with tooling that
		// may not have an interactive session, e.g. installing from Ansible.
		if *fService != "" {
			if *fConfig != "" {
				svcConfig.Arguments = []string{"--config", *fConfig}
			}
			if *fConfigDirectory != "" {
				svcConfig.Arguments = append(svcConfig.Arguments, "--config-directory", *fConfigDirectory)
			}
			//set servicename to service cmd line, to have a custom name after relaunch as a service
			svcConfig.Arguments = append(svcConfig.Arguments, "--service-name", *fServiceName)

			err := service.Control(s, *fService)
			if err != nil {
				log.Fatal("E! " + err.Error())
			}
			os.Exit(0)
		} else {
			// When in service mode, register eventlog target and setup default logging to eventlog
			e := RegisterEventLogger()
			if e != nil {
				log.Println("E! Cannot register event log " + e.Error())
			}
			err = s.Run()

			if err != nil {
				log.Println("E! " + err.Error())
			}
		}
	} else {
		stop = make(chan struct{})
		reloadLoop(
			stop,
			inputFilters,
			outputFilters,
			aggregatorFilters,
			processorFilters,
		)
	}
}

// Return true if Telegraf should create a Windows service.
func windowsRunAsService() bool {
	if *fService != "" {
		return true
	}

	if *fRunAsConsole {
		return false
	}

	return !service.Interactive()
}

func loadTomlConfigIntoAgent(c *config.Config) error {
	isOld, err := migrate.IsOldConfig(*fConfig)
	if err != nil {
		log.Printf("W! Failed to detect if config file is old format: %v", err)
	}

	if isOld {
		migratedConfFile, err := migrate.MigrateFile(*fConfig)
		if err != nil {
			log.Printf("W! Failed to migrate old config format file %v: %v", *fConfig, err)
		}

		err = c.LoadConfig(migratedConfFile)
		if err != nil {
			return err
		}

		agentinfo.BuildStr += "_M"
	} else {
		err = c.LoadConfig(*fConfig)
		if err != nil {
			return err
		}
	}

	if *fConfigDirectory != "" {
		err = c.LoadDirectory(*fConfigDirectory)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateAgentFinalConfigAndPlugins(c *config.Config) error {
	if !*fTest && len(c.Outputs) == 0 {
		return errors.New("Error: no outputs found, did you provide a valid config file?")
	}
	if len(c.Inputs) == 0 {
		return errors.New("Error: no inputs found, did you provide a valid config file?")
	}

	if int64(c.Agent.Interval) <= 0 {
		return fmt.Errorf("Agent interval must be positive, found %v", c.Agent.Interval)
	}

	if int64(c.Agent.FlushInterval) <= 0 {
		return fmt.Errorf("Agent flush_interval must be positive; found %v", c.Agent.FlushInterval)
	}

	if inputPlugin, err := checkRightForBinariesFileWithInputPlugins(c.InputNames()); err != nil {
		return fmt.Errorf("Validate input plugin %s failed because of %v", inputPlugin, err)
	}

	if *fSchemaTest {
		//up to this point, the given config file must be valid
		fmt.Println(agentinfo.FullVersion())
		fmt.Printf("The given config: %v is valid\n", *fConfig)
		os.Exit(0)
	}

	return nil
}

func checkRightForBinariesFileWithInputPlugins(inputPlugins []string) (string, error) {
	for _, inputPlugin := range inputPlugins {
		if inputPlugin == "nvidia_smi" {
			if err := internal.CheckNvidiaSMIBinaryRights(); err != nil {
				return "nvidia_smi", err
			}
		}
	}

	return "", nil
}
