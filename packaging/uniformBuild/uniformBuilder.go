// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"uniformBuild/common"
	"uniformBuild/remoteBuilder"

	"golang.org/x/sync/errgroup"
)

func main() {
	var repo string
	var branch string
	var bucketKey string
	var packageBucketKey string
	var accountID string
	flag.StringVar(&repo, "r", "", "repository")
	flag.StringVar(&repo, "repo", "", "repository")
	flag.StringVar(&branch, "b", "", "branch")
	flag.StringVar(&branch, "branch", "", "branch")
	flag.StringVar(&bucketKey, "o", "", "bucketKey")
	flag.StringVar(&bucketKey, "bucketKey", "", "bucketKey")
	flag.StringVar(&packageBucketKey, "p", "", "packageBucketKey")
	flag.StringVar(&packageBucketKey, "packageBucketKey", "", "packageBucketKey")
	flag.StringVar(&accountID, "a", "", "accountID")
	flag.StringVar(&accountID, "account_id", "", "accountID")
	flag.Parse()
	rbm := remoteBuilder.CreateRemoteBuildManager(common.DEFAULT_INSTANCE_GUIDE, accountID)
	//@TODO add a cache check where it doesn't create a instance if functions that have use it are complete
	var err error
	eg := new(errgroup.Group)
	defer rbm.Close()
	eg.Go(func() error {
		return rbm.MakeMacPkg("MacPkgMaker", packageBucketKey)
	})
	err = rbm.BuildCWAAgent(repo, branch, bucketKey, "MainBuildEnv")
	if err != nil {
		panic(err)
	}
	eg.Go(func() error { // windows
		err = rbm.MakeMsiZip("WindowsMSIPacker", bucketKey)
		if err != nil {
			return err
		}
		err = rbm.BuildMSI("WindowsMSIBuilder", bucketKey, packageBucketKey)
		if err != nil {
			return err
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		fmt.Printf("Failed because: %s \n", err)
		return
	}
	fmt.Printf("\033[32mSuccesfully\033[0m built CWA from %s with %s branch, check \033[32m%s \033[0m bucket with \033[1;32m%s\033[0m hash\n",
		repo, branch, common.S3_INTEGRATION_BUCKET, bucketKey)

}
