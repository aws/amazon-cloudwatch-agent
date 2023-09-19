
# Uniform Build Enviorment Docs


## Features
- Detect latest AMI and retrieve the image
- Generate an individual ec2 instance for each AMI (linux,windows,mac)
  - This generation will wait until all instances are running.
  - This generation will start the instance generation concurrently 
  - The instances are kept track on individual goroutines during their launch
  - These goroutines are killed after all instances are running.
- Run commands on an ec2 instance
  - Possible commands that can be run currently are as follows:
    - GitClone a repo 
    - Call MakeBuild which builds the agent
    - UploadS3 which uploads the build files to a specific s3 bucket
    - RemoveFolder which lets you remove a file or folder 
    - Note: Commands can be added easily since they are just strings more info in: [How to add commands](#how-to-add-commands)
- Clone, build, and upload CWA linux build
## How to Run
A. If you are running on git actions with your fork follow [this]() tutorial
> [!WARNING]
> This is not recommended, it is highly recommended to just use either local setup or official git repos.

B. Running locally you follow this tutorial:
1. First setup git locally (gh-cli is recommended)
2. Secondly setup your aws-credentials 
3. In your aws account follow the [setup](#setup) steps 
4. After you environment is fully setup, clone this repo and run the following script to  build the go script:
```shell
cd packaging/uniformBuild && go build .
```
5. Now run the following command to start the build:
> [!WARNING]
> This can take upto 20 minutes.
> If you close the terminal after the command is sent, it will still build but you cannot track it in the terminal.
> If that has happened you can go to ssm's website and go to run command to track your build.
```shell
./uniformBuild -r {THE REPO YOU WANT TO BUILD'S URL }-b {THE BRANCH YOU WANT TO BUILD} -c {COMMENT TO DISTINGUISH YOUR BUILD COMMAND (optional}
```
6. 
C. Running remotely on a ec2 instance:
1. tbd
## Setup
### AWS Tools Needed:
- EC2 Image Builder
  - EC2 Image Pipeline
  - EC2 Image Recipe 
  - EC2 Components[^1]:
    - CWA_RPM_BUILD
    - CWA_INSTALL_GOLANG
    - CWA_OTHER_DEP
- SSM 
- IAM Roles
- S3 Bucket
### Instructions:
#### 1. Creating Custom AMI[^2]
1. Go to EC2  Image builder click on **Components** under Saved configurations.
2. Click on **Create component**
3. We will be creating 3 components as shown in : [AWS Tools needed](#aws-tools-needed)
4. Creating any component
   - For type pick **build**
   - Name the component (remember the name) 
   - Give 1.0.0 for version number
   - Pick the OS to your AMI's OS (for our case this is Ubuntu 22 LTS)
   - Leave everything else default
   - For content of each component goto  [components](components) folder and copy the file content for the component you are creating. For example for copy [CWA_Install_Golang.yaml](components/CWA-Install-Golang.yaml) for Go install component.
   - Hit **Create Component** button on bottom of the page
5. Go to EC2  Image builder click on **Image recipes** under Saved configurations.
6. Click on **Create Image Recipe**
7. Creating the recipe:
   - You can name this recipe anything you want; however, CWA_Build_Env is suggested.
   - Name the version to 1.0.0
   - Pick your prefered OS ( for this tutorial it will be Ubuntu) 
   > [!Important]
   > Make sure the OS you have picked is the latest version (LTS)
   - Now add the components you have created to the recipe
   > [!Important]
   > Make sure you have edited the component's version as latest( it is not by default)
   - Hit **Create Recipe** button
8. Now click on **Image pipelines** under Saved configurations
9. Click on Create image pipeline.
10. Creating the image pipeline:
    - For general details you can label as you please
    - For schedule pick manual
    - For recipe choose the recipe you just have created
    - Leave everything else as default.
11. Now Open your new pipeline, go to actions, and hit **Run pipeline**
12. After the image is built successfully continue to next step
#### 2. Setting up the IAM Roles & SSM
1. You can follow this tutorial [official remotely run commands on ec2](https://aws.amazon.com/getting-started/hands-on/remotely-run-commands-ec2-instance-systems-manager/)
#### 3. Setting up the S3 bucket

[^1]: We have RPM and Golang as seperate componenets because they may need updates more frequently or require more control over the build version
[^2]: This tutorial will be showing how to setup a Ubuntu AMI but it can be applied to any OS
Pipeline
---
## How to add commands
---
## AMI
### Package dep. list:
#### Linux: 
- [X] Golang
- [X] Rpm-build
- [ ] Zip
- [ ] Docker
- [ ] Qemu
- [X] aws
---
## Goals
- [ ] Add windows and MacOS amis
- [ ] Optimize GoBuild with S3 caching
---
## TODO
- [ ] Clean up file structure
- [ ] try installing with the snap install instead of apt-get to see if you dont need extra stuff
- [ ] Pull latest amis directly
- [ ] Add non-blocking run commands
- WindowsMSIPacker
- MSIUpload
- MACOS AMI
---
