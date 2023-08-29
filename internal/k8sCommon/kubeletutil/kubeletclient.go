// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package kubeletutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent/internal/tls"
)

type KubeClient struct {
	Port            string
	BearerToken     string
	KubeIP          string
	responseTimeout time.Duration
	roundTripper    http.RoundTripper
	tls.ClientConfig
}

var ErrKubeClientAccessFailure = errors.New("KubeClient Access Failure")

func (k *KubeClient) ListPods() ([]corev1.Pod, error) {
	var result []corev1.Pod
	url := fmt.Sprintf("https://%s:%s/pods", k.KubeIP, k.Port)

	var req, _ = http.NewRequest("GET", url, nil)
	var resp *http.Response

	k.InsecureSkipVerify = true
	tlsCfg, err := k.ClientConfig.TLSConfig()
	if err != nil {
		return result, err
	}

	if k.roundTripper == nil {
		// Set default values
		if k.responseTimeout < time.Second {
			k.responseTimeout = time.Second * 5
		}
		k.roundTripper = &http.Transport{
			TLSHandshakeTimeout:   5 * time.Second,
			TLSClientConfig:       tlsCfg,
			ResponseHeaderTimeout: k.responseTimeout,
		}
	}

	if k.BearerToken != "" {
		token, err := os.ReadFile(k.BearerToken)
		if err != nil {
			return result, err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	}
	req.Header.Add("Accept", "application/json")

	resp, err = k.roundTripper.RoundTrip(req)
	if err != nil {
		log.Printf("E! error making HTTP request to %s: %s", url, err)
		return result, ErrKubeClientAccessFailure
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("E! %s returned HTTP status %s", url, resp.Status)
		return result, ErrKubeClientAccessFailure
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("E! Fail to read request %s body: %s", url, err)
		return result, err
	}

	pods := corev1.PodList{}
	err = json.Unmarshal(b, &pods)
	if err != nil {
		log.Printf("E! parsing response: %s", err)
		return result, err
	}

	return pods.Items, nil
}
