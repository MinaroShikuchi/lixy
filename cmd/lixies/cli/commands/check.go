package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check agent system requirements",
	Long:  `Check if Docker/Podman and other required tools are installed`,
}

var checkRuntimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Check container runtime (Docker/Podman)",
	Long:  `Verify if Docker or Podman is installed and working`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Checking container runtime...")
		fmt.Println()

		// Check Docker
		dockerWorking := false
		if _, err := exec.LookPath("docker"); err == nil {
			fmt.Println("✓ Docker binary found")

			if err := exec.Command("docker", "version").Run(); err == nil {
				dockerWorking = true
				fmt.Println("✓ Docker is working")

				// Get Docker version
				out, _ := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
				if len(out) > 0 {
					fmt.Printf("  Version: %s", string(out))
				}
			} else {
				fmt.Println("✗ Docker binary found but not working")
				fmt.Printf("  Error: %v\n", err)
			}
		} else {
			fmt.Println("✗ Docker not found")
		}
		fmt.Println()

		// Check Podman
		podmanWorking := false
		if _, err := exec.LookPath("podman"); err == nil {
			fmt.Println("✓ Podman binary found")

			if err := exec.Command("podman", "version").Run(); err == nil {
				podmanWorking = true
				fmt.Println("✓ Podman is working")

				// Get Podman version
				out, _ := exec.Command("podman", "version", "--format", "{{.Version}}").Output()
				if len(out) > 0 {
					fmt.Printf("  Version: %s", string(out))
				}
			} else {
				fmt.Println("✗ Podman binary found but not working")
				fmt.Printf("  Error: %v\n", err)
			}
		} else {
			fmt.Println("✗ Podman not found")
		}
		fmt.Println()

		// Check Docker Compose
		fmt.Println("Checking compose tools...")
		composeFound := false

		// Docker compose plugin
		if dockerWorking {
			if err := exec.Command("docker", "compose", "version").Run(); err == nil {
				fmt.Println("✓ Docker Compose (plugin) found")
				out, _ := exec.Command("docker", "compose", "version", "--short").Output()
				if len(out) > 0 {
					fmt.Printf("  Version: %s", string(out))
				}
				composeFound = true
			}
		}

		// Standalone docker-compose
		if _, err := exec.LookPath("docker-compose"); err == nil {
			if err := exec.Command("docker-compose", "--version").Run(); err == nil {
				fmt.Println("✓ Docker Compose (standalone) found")
				out, _ := exec.Command("docker-compose", "--version").Output()
				if len(out) > 0 {
					fmt.Printf("  %s", string(out))
				}
				composeFound = true
			}
		}

		// Podman compose
		if podmanWorking {
			// Try podman compose plugin first
			if err := exec.Command("podman", "compose", "version").Run(); err == nil {
				fmt.Println("✓ Podman Compose (plugin) found")
				out, _ := exec.Command("podman", "compose", "version").Output()
				if len(out) > 0 {
					fmt.Printf("  %s", string(out))
				}
				composeFound = true
			} else if _, err := exec.LookPath("podman-compose"); err == nil {
				// Fallback to standalone podman-compose
				if err := exec.Command("podman-compose", "--version").Run(); err == nil {
					fmt.Println("✓ Podman Compose (standalone) found")
					out, _ := exec.Command("podman-compose", "--version").Output()
					if len(out) > 0 {
						fmt.Printf("  %s", string(out))
					}
					composeFound = true
				}
			}

			// Check Podman socket status (important for Docker compatibility)
			fmt.Println("\nPodman Socket:")
			if runtime.GOOS == "darwin" {
				// macOS: Check podman machine
				cmd := exec.Command("podman", "machine", "inspect", "--format", "{{.State}}")
				if output, err := cmd.Output(); err == nil {
					state := strings.TrimSpace(string(output))
					if state == "running" {
						fmt.Println("✓ Podman machine is running")

						// Get socket path
						socketCmd := exec.Command("podman", "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}")
						if socketOutput, err := socketCmd.Output(); err == nil {
							socketPath := strings.TrimSpace(string(socketOutput))
							fmt.Printf("  Socket: %s\n", socketPath)

							// Check DOCKER_HOST
							dockerHost := os.Getenv("DOCKER_HOST")
							if dockerHost != "" {
								fmt.Printf("  DOCKER_HOST: %s\n", dockerHost)
							} else {
								fmt.Printf("  ⚠ DOCKER_HOST not set (agent will configure this)\n")
							}
						}
					} else {
						fmt.Printf("✗ Podman machine is %s (should be running)\n", state)
						fmt.Println("  Run: podman machine start")
					}
				} else {
					fmt.Println("✗ Could not check podman machine status")
				}
			} else {
				// Linux: Check systemd socket
				runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
				if runtimeDir != "" {
					socketPath := filepath.Join(runtimeDir, "podman", "podman.sock")
					if _, err := os.Stat(socketPath); err == nil {
						fmt.Printf("✓ Podman socket found at %s\n", socketPath)

						dockerHost := os.Getenv("DOCKER_HOST")
						if dockerHost != "" {
							fmt.Printf("  DOCKER_HOST: %s\n", dockerHost)
						} else {
							fmt.Printf("  ⚠ DOCKER_HOST not set (agent will configure this)\n")
						}
					} else {
						fmt.Println("✗ Podman socket not found")
						fmt.Println("  Run: systemctl --user start podman.socket")
					}
				}
			}
		}

		if !composeFound {
			fmt.Println("✗ No compose tool found")
		}
		fmt.Println()

		// Summary
		fmt.Println("Summary:")
		if dockerWorking || podmanWorking {
			fmt.Println("✓ Container runtime available")
			if dockerWorking {
				fmt.Println("  Recommended: Docker")
			} else {
				fmt.Println("  Using: Podman")
			}
		} else {
			fmt.Println("✗ No working container runtime found")
			fmt.Println("  Please install Docker or Podman")
			return fmt.Errorf("container runtime required")
		}

		if !composeFound {
			fmt.Println("⚠ Compose tool not found")
			fmt.Println("  Install docker-compose or podman-compose for deployment support")
		}

		return nil
	},
}

func init() {
	checkCmd.AddCommand(checkRuntimeCmd)
}
