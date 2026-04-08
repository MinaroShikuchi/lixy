package daemon

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

const systemdUnitTemplate = `[Unit]
Description={{.DisplayName}}
After=network.target
Wants=network-online.target

[Service]
Type=simple
User={{.User}}
Group={{.Group}}
WorkingDirectory={{.InstallDir}}
ExecStart={{.InstallDir}}/{{.BinaryName}}
Restart=on-failure
RestartSec=10
StandardOutput=append:{{.LogDir}}/{{.ServiceName}}.log
StandardError=append:{{.LogDir}}/{{.ServiceName}}-error.log
SyslogIdentifier={{.ServiceName}}
EnvironmentFile=-{{.ConfigDir}}/{{.ServiceName}}.env
{{- range $key, $value := .EnvVars}}
Environment="{{$key}}={{$value}}"
{{- end}}

LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`

// generateUnitFile renders the systemd unit file from the DaemonConfig.
func generateUnitFile(cfg DaemonConfig) (string, error) {
	tmpl, err := template.New("systemd").Parse(systemdUnitTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse systemd template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("failed to render systemd template: %w", err)
	}

	return buf.String(), nil
}

// writeUnitFile generates and writes the systemd unit file to /etc/systemd/system/.
func writeUnitFile(cfg DaemonConfig) error {
	content, err := generateUnitFile(cfg)
	if err != nil {
		return err
	}

	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", cfg.ServiceName)
	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write unit file to %s: %w", unitPath, err)
	}

	return nil
}
