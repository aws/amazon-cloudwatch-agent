<?xml version='1.0' encoding='UTF-8'?>
<Wix xmlns='http://schemas.microsoft.com/wix/2006/wi'
     xmlns:util='http://schemas.microsoft.com/wix/UtilExtension'>

  <Product Id='*'
    Name='Amazon CloudWatch Agent'
    UpgradeCode='c537c936-91b3-4270-94d7-e128acfc3e86'
    Language='1033'
    Codepage='1252'
    Version='<version>'
    Manufacturer='Amazon.com, Inc.'>

    <Package Id='*'
      Keywords='Installer'
      Description="Amazon CloudWatch Agent Installer"
      Comments='Copyright 2018 Amazon.com, Inc. and its affiliates. All Rights Reserved.'
      Manufacturer='Amazon.com, Inc.'
      InstallerVersion='200'
      Languages='1033'
      Compressed='yes'
      SummaryCodepage='1252'
      InstallScope="perMachine"
      Platform="x64"
    />

    <MediaTemplate EmbedCab='yes' />
    <Property Id="POWERSHELLEXE">
      <RegistrySearch Id="POWERSHELLEXE"
                      Type="raw"
                      Root="HKLM"
                      Key="SOFTWARE\Microsoft\PowerShell\1\ShellIds\Microsoft.PowerShell"
                      Name="Path" />
    </Property>
    <Feature Id='ProductFeature' Title="Amazon CloudWatch Agent" Level='1'>
      <ComponentRef Id='StarterEXE' />
      <ComponentRef Id='AgentEXE' />
      <ComponentRef Id='WizardEXE' />
      <ComponentRef Id='Ctl' />
      <ComponentRef Id='SchemaJSON' />
      <ComponentRef Id='DownloaderEXE' />
      <ComponentRef Id='TranslatorEXE' />
      <ComponentRef Id='CWAGENT_VERSION' />
      <ComponentRef Id='LICENSE' />
      <ComponentRef Id='NOTICE' />
      <ComponentRef Id='RELEASE_NOTES' />
      <ComponentRef Id='THIRD_PARTY_LICENSES' />
      <ComponentRef Id='CommonConfigTOML' />
      <ComponentRef Id='CreateLogsFolder' />
      <ComponentRef Id='CreateConfigsFolder' />
      <ComponentRef Id='CreateCWOCConfigsFolder' />
      <ComponentRef Id='CreateCWOCLogsFolder' />
      <ComponentRef Id='CWOCEXE' />
      <ComponentRef Id='PredefinedConfigData' />
      <ComponentRef Id='FIX_PERMISSION' />
    </Feature>

    <Directory Id='TARGETDIR' Name='SourceDir'>

      <Directory Id='ProgramFiles64Folder'>
        <Directory Id='PFilesAmazon' Name='Amazon'>
          <Directory Id='INSTALLDIR' Name='AmazonCloudWatchAgent'/>
        </Directory>
      </Directory>

      <Directory Id='CommonAppDataFolder' Name='AppDataFolder'>
        <Directory Id='AppDataFolderAmazon' Name='Amazon'>
          <Directory Id='Config' Name='AmazonCloudWatchAgent'>
            <Directory Id="Configs" Name="Configs"/>
            <Directory Id='Logs' Name='Logs'/>
            <Directory Id='CWOCConfig' Name='CWAgentOtelCollector'>
              <Directory Id='CWOCConfigs' Name='Configs'/>
              <Directory Id='CWOCLogs' Name='Logs'/>
            </Directory>
          </Directory>
        </Directory>
      </Directory>

    </Directory>

    <DirectoryRef Id="INSTALLDIR">
        <Component Id='StarterEXE' Guid='5f344c26-c8f5-4a10-83c0-0651399fb8ff' Win64='yes'>
            <File Source='start-amazon-cloudwatch-agent.exe' KeyPath='yes' Checksum='yes'/>
            <ServiceInstall
                Id="ServiceInstaller"
                Type="ownProcess"
                Name="AmazonCloudWatchAgent"
                DisplayName="Amazon CloudWatch Agent"
                Description="Amazon CloudWatch Agent"
                Start="auto"
                Account="LocalSystem"
                Interactive="no"
                ErrorControl="normal"
                Vital="yes"
            >
                <ServiceDependency Id="LanmanServer"/>
                <ServiceConfig FirstFailureActionType="restart" SecondFailureActionType="restart" ThirdFailureActionType="restart" ResetPeriodInDays="1" RestartServiceDelayInSeconds="2" xmlns="http://schemas.microsoft.com/wix/UtilExtension"/>
                <ServiceConfig OnInstall="yes" OnReinstall="yes" FailureActionsWhen="failedToStopOrReturnedError"/>
            </ServiceInstall>
            <ServiceControl
                Id="StartService"
                Stop="both"
                Remove="uninstall"
                Name="AmazonCloudWatchAgent"
                Wait="yes"
            />
        </Component>
        <Component Id='CWOCEXE' Guid='3afd22e7-3f83-413f-9861-e1ac923a15c4' Win64='yes'>
            <File Source='cwagent-otel-collector.exe' KeyPath='yes' Checksum='yes'/>
            <ServiceInstall
                Id="CWOCServiceInstaller"
                Type="ownProcess"
                Name="CWAgentOtelCollector"
                DisplayName="CWAgent Otel Collector"
                Description="CWAgent Otel Collector"
                Start="demand"
                Account="LocalSystem"
                Interactive="no"
                ErrorControl="normal"
                Arguments=" --config=&quot;[CWOCConfig]cwagent-otel-collector.yaml&quot;"
                Vital="yes"
            >
                <ServiceDependency Id="LanmanServer"/>
                <ServiceConfig FirstFailureActionType="restart" SecondFailureActionType="restart" ThirdFailureActionType="restart" ResetPeriodInDays="1" RestartServiceDelayInSeconds="2" xmlns="http://schemas.microsoft.com/wix/UtilExtension"/>
                <ServiceConfig OnInstall="yes" OnReinstall="yes" FailureActionsWhen="failedToStopOrReturnedError"/>
            </ServiceInstall>
            <ServiceControl
                Id="CWOCStartService"
                Stop="both"
                Remove="uninstall"
                Name="CWAgentOtelCollector"
                Wait="yes"
            />
        </Component>
        <Component Id='AgentEXE' Guid='d98c86be-b6c8-4f24-84a5-03b08bd6e7f2' Win64='yes'>
            <File Source='amazon-cloudwatch-agent.exe' KeyPath='yes' Checksum='yes'/>
        </Component>
        <Component Id='WizardEXE' Guid='e8c20fcf-94c7-4097-97ed-ef4cc5c867b2' Win64='yes'>
            <File Source='amazon-cloudwatch-agent-config-wizard.exe' KeyPath='yes' Checksum='yes'/>
        </Component>
        <Component Id='Ctl' Guid='f95f122b-aa48-4f6e-beab-05380b8ce99d' Win64='yes'>
            <File Source='amazon-cloudwatch-agent-ctl.ps1' KeyPath='yes'/>
        </Component>
        <Component Id='SchemaJSON' Guid='80a1bfcc-8a0f-46e2-8e84-c2023d10fdf3' Win64='yes'>
            <File Source='amazon-cloudwatch-agent-schema.json' KeyPath='yes'/>
        </Component>
        <Component Id='DownloaderEXE' Guid='727f4d1b-76bd-4cde-969a-02f16e4425ac' Win64='yes'>
            <File Source='config-downloader.exe' KeyPath='yes' Checksum='yes'/>
        </Component>
        <Component Id='TranslatorEXE' Guid='f4527006-edcb-4271-a971-039848bc8bb7' Win64='yes'>
            <File Source='config-translator.exe' KeyPath='yes' Checksum='yes'/>
        </Component>
        <Component Id='CWAGENT_VERSION' Guid='f4ddf7bf-48fc-41f6-a914-4153a7cf0afc' Win64='yes'>
            <File Source='CWAGENT_VERSION' KeyPath='yes'/>
        </Component>
        <Component Id='LICENSE' Guid='ac70ef6c-8ec4-4a91-8059-2c18543df863' Win64='yes'>
            <File Source='LICENSE' KeyPath='yes'/>
        </Component>
        <Component Id='NOTICE' Guid='d490c48d-eed1-445d-8eac-99769c472ec7' Win64='yes'>
            <File Source='NOTICE' KeyPath='yes'/>
        </Component>
        <Component Id='RELEASE_NOTES' Guid='5bb03e58-44e1-4acc-a827-ad91e25025b9' Win64='yes'>
            <File Source='RELEASE_NOTES' KeyPath='yes'/>
        </Component>
        <Component Id='THIRD_PARTY_LICENSES' Guid='ca4ac31e-8c1d-482f-9724-27f8857caca5' Win64='yes'>
            <File Source='THIRD-PARTY-LICENSES' KeyPath='yes'/>
        </Component>
        <Component Id='FIX_PERMISSION' Guid='6ea35ac1-b8cc-492b-b62f-312c30395110' Win64='yes'>
            <File Source='permission.ps1' KeyPath='yes'/>
        </Component>
    </DirectoryRef>

    <DirectoryRef Id="Config">
        <Component Id='CommonConfigTOML' Guid='293f73c5-1f51-4e65-86e3-97425ec75c94' Win64='yes' NeverOverwrite='yes' Permanent='yes'>
            <File Source='common-config.toml' KeyPath='yes'/>
        </Component>
    </DirectoryRef>

    <DirectoryRef Id="Configs">
        <Component Id='CreateConfigsFolder' Guid='c860d000-ed10-11e8-8eb2-f2801f1b9fd1' Win64='yes'>
            <CreateFolder />
        </Component>
    </DirectoryRef>

    <DirectoryRef Id="CWOCConfig">
        <Component Id='PredefinedConfigData' Guid='b0543a32-51e2-4f89-8375-4924e46095f4' Win64='yes' NeverOverwrite='yes' Permanent='yes'>
            <File Source='predefined-config-data' KeyPath='yes'/>
        </Component>
    </DirectoryRef>

    <DirectoryRef Id="CWOCConfigs">
        <Component Id='CreateCWOCConfigsFolder' Guid='8c7cb53c-9b56-47b7-8a06-7c164a0b574a' Win64='yes'>
            <CreateFolder />
        </Component>
    </DirectoryRef>

    <DirectoryRef Id="CWOCLogs">
        <Component Id='CreateCWOCLogsFolder' Guid='bfbfaece-1a9a-489b-bf1c-1039a7f70803' Win64='yes'>
            <CreateFolder />
        </Component>
    </DirectoryRef>

    <DirectoryRef Id="Logs">
        <Component Id='CreateLogsFolder' Guid='fe9042cb-a4fa-4b8e-9852-685a342338b5' Win64='yes'>
            <CreateFolder />
        </Component>
    </DirectoryRef>
     <!-- Find and use powershell to run the command, because just running "powershell.exe" did not resolve (not in ENV path) when using "WixQuietExec".-->
    <SetProperty Id="QtExecUpdateConfigPermission" 
        Sequence="execute"
        Before ="QtExecUpdateConfigPermission"
        Value='&quot;[POWERSHELLEXE]&quot;  -ExecutionPolicy Bypass -File "[INSTALLDIR]permission.ps1" ' 
    />
    <!-- Setup a silent execution contrainer around the command -->
    <CustomAction Id="QtExecUpdateConfigPermission" 
    BinaryKey="WixCA" 
    DllEntry="WixQuietExec" 
    Execute="deferred" 
    Return="check" 
    Impersonate="no" />

    <InstallExecuteSequence>
    <Custom Action="QtExecUpdateConfigPermission" After="InstallFiles">NOT UPGRADINGPRODUCTCODE AND NOT (REMOVE~="ALL")</Custom>
    </InstallExecuteSequence>

    <MajorUpgrade AllowDowngrades="yes"/>
  </Product>
</Wix>