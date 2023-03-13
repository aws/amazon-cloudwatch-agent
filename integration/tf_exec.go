package integration

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
	"log"
	"os"
)

const testDir = "/tmp/amazon-cloudwatch-agent-test"

func RunIntegrationTest(integConfig IntegConfig, varsAbsolutePath string) {
	err := removeTestSuite()
	if err != nil {
		log.Fatal("Error removeTestSuite(): ", err)
	}

	s3Bucket, cwaGithubSha := validateRunIntegConfig(integConfig)
	err = cloneTestSuite(s3Bucket, cwaGithubSha)
	if err != nil {
		log.Fatal("Error cloneTestSuite(): ", err)
	}

	//if terraformRelativePath, ok := integConfig["terraformRelativePath"].(string); ok {
	//	terraformAbsolutePath := path.Join(rootDir, terraformRelativePath)
	//} else {
	//	log.Fatal("Error: terraformPath was not provided in config.json")
	//}
}

func removeTestSuite() error {
	log.Print("Remove old test repo")
	args := fmt.Sprintf("-Rf %v", testDir)
	err := ExecCommandWithStderr("rm", args)
	return err
}

func validateRunIntegConfig(integConfig IntegConfig) (string, string) {
	s3Bucket, ok := integConfig["githubTestRepo"].(string)
	if !ok {
		log.Fatal("Error: githubTestRepo was not provided in integConfig.json")
	}

	cwaGithubSha, ok := integConfig["githubTestRepoBranch"].(string)
	if !ok {
		log.Fatal("Error: githubTestRepoBranch was not provided in integConfig.json")
	}
	return s3Bucket, cwaGithubSha
}

func cloneTestSuite(githubTestRepo, githubTestRepoBranch string) error {
	log.Print("Cloning test repo")
	args := fmt.Sprintf("clone -b %v %v %v", githubTestRepoBranch, githubTestRepo, testDir)
	fmt.Println("git", args)
	err := ExecCommandWithStderr("git", args)
	return err
}

func terraformApply(terraformAbsolutePath, varsAbsolutePath string) {
	fmt.Println("Running terraform suite =", terraformAbsolutePath)
	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion("1.0.6")),
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		log.Fatalf("error installing Terraform: %s", err)
	}

	tf, err := tfexec.NewTerraform(terraformAbsolutePath, execPath)
	if err != nil {
		log.Fatalf("error running NewTerraform: %s", err)
	}

	// log everything
	tf.SetStderr(os.Stderr)
	tf.SetStdout(os.Stdout)

	// terraform init
	err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		log.Fatalf("error running Init: %s", err)
	}

	// terraform apply --auto-approve -var-file="${varsAbsolutePath}"
	err = tf.Apply(context.Background(), tfexec.VarFile(varsAbsolutePath))
	if err != nil {
		log.Fatalf("error running tf.Apply(): %s", err)
	}

	// terraform destroy --auto-approve
	err = tf.Destroy(context.Background())
	if err != nil {
		log.Fatalf("error running tf.Destroy(): %s", err)
	}
}
