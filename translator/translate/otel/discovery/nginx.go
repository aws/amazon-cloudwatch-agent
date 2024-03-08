package discovery

import (
	"github.com/aws/amazon-cloudwatch-agent/translator/translate/otel/common"
	"os"
	"os/exec"
	"regexp"
)

type discoverer struct {
}

func NginxDiscoverer() common.Discoverer {
	return &discoverer{}
}

func (t *discoverer) Discover() (map[string]any, error) {
	_, err := getNginxConfPath()
	if err != nil {
		return map[string]any{}, nil
	}
	/*nginxConf, err := getNginxConfPath()
	if err != nil {
		return map[string]any{}, nil
	}
	p, err := parser.NewParser(nginxConf, parser.WithSkipComments())
	c, err := p.Parse()
	ss := c.FindDirectives("stub_status")
	if len(ss) == 0 {
		server := c.FindDirectives("server")
		fmt.Print(server)
	}*/
	file, err := os.Create("/etc/nginx/conf.d/stub_status.conf")
	defer file.Close()
	_, err = file.WriteString("server {\n    listen 127.0.0.1:81;\n    server_name 127.0.0.1;\n    location /nginx_status {\n        stub_status on;\n        allow 127.0.0.1;\n        deny all;\n    }\n}")
	if err != nil {
		return map[string]any{}, nil
	}

	cmd := exec.Command("sudo", "nginx", "-s", "reload")
	err = cmd.Run()
	if err != nil {
		return map[string]any{}, err
	}

	return map[string]any{
		"metrics": map[string]any{
			"metrics_collected": map[string]any{
				"nginx": map[string]any{
					"endpoint": "http://127.0.0.1:81/nginx_status",
				},
			},
		},
	}, nil
}

func (t *discoverer) ID() string {
	return "nginx"
}

func getNginxConfPath() (string, error) {
	cmd := exec.Command("sudo", "nginx", "-t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`.*?the configuration file (.+) syntax is ok.*`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) != 2 {
		return "", err
	}
	return matches[1], nil
}
