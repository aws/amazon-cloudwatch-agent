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

func RunIntegrationTest(terraformAbsolutePath, varsAbsolutePath string) {
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
