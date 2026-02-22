package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/stevenmeow/clawpeteer-agent/buildcfg"
	"github.com/stevenmeow/clawpeteer-agent/certs"
	"github.com/stevenmeow/clawpeteer-agent/internal/config"
	"github.com/stevenmeow/clawpeteer-agent/internal/handler"
	"github.com/stevenmeow/clawpeteer-agent/internal/security"
	"github.com/stevenmeow/clawpeteer-agent/internal/taskmanager"
)

func main() {
	// CLI flags
	configPath := flag.String("config", "", "path to configuration file")
	flagID := flag.String("id", "", "agent ID (overrides config)")
	flagBroker := flag.String("broker", "", "MQTT broker URL (overrides config)")
	flagUser := flag.String("user", "", "MQTT username (overrides config)")
	flagPass := flag.String("pass", "", "MQTT password (overrides config)")
	flagCAFile := flag.String("ca", "", "CA certificate file path (overrides config)")
	flag.Parse()

	// Load config with priority: file flag > embedded > defaults
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// CLI flags override config values
	if *flagID != "" {
		cfg.AgentID = *flagID
	}
	if *flagBroker != "" {
		cfg.Broker.URL = *flagBroker
	}
	if *flagUser != "" {
		cfg.Broker.Username = *flagUser
	}
	if *flagPass != "" {
		cfg.Broker.Password = *flagPass
	}
	if *flagCAFile != "" {
		cfg.Broker.CAFile = *flagCAFile
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	log.Printf("Clawpeteer Agent starting (id=%s)", cfg.AgentID)

	// Build MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker.URL)
	opts.SetClientID(fmt.Sprintf("%s-%s", cfg.AgentID, uuid.New().String()[:8]))
	opts.SetUsername(cfg.Broker.Username)
	opts.SetPassword(cfg.Broker.Password)

	// TLS configuration
	if strings.HasPrefix(cfg.Broker.URL, "mqtts://") || strings.HasPrefix(cfg.Broker.URL, "ssl://") {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		var caCert []byte

		if cfg.Broker.CAFile != "" {
			// Priority 1: config file path
			caCert, err = os.ReadFile(cfg.Broker.CAFile)
			if err != nil {
				log.Fatalf("Failed to read CA file: %v", err)
			}
			log.Println("Using CA certificate from config file")
		} else if embedded := certs.LoadEmbeddedCA(); embedded != nil {
			// Priority 2: embedded at build time
			caCert = embedded
			log.Println("Using embedded CA certificate")
		}

		if caCert != nil {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				log.Fatal("Failed to parse CA certificate")
			}
			tlsCfg.RootCAs = caCertPool
		} else {
			log.Println("No CA certificate provided, using system root CAs")
		}

		opts.SetTLSConfig(tlsCfg)
	}

	// Last Will and Testament
	willTopic := fmt.Sprintf("clawpeteer/%s/status", cfg.AgentID)
	opts.SetWill(willTopic, `{"status":"offline"}`, 1, true)

	// Auto-reconnect
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetKeepAlive(time.Duration(cfg.HeartbeatInterval) * time.Second)

	// Connection handlers
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("Connected to MQTT broker: %s", cfg.Broker.URL)
		// Publish online status
		statusTopic := fmt.Sprintf("clawpeteer/%s/status", cfg.AgentID)
		c.Publish(statusTopic, 1, true, `{"status":"online"}`)
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})
	opts.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		log.Println("Reconnecting to MQTT broker...")
	})

	// Connect
	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}

	// Wire handler, security, and task manager
	secValidator := security.New(cfg.Security.Mode, cfg.Security.Whitelist, cfg.Security.Blacklist, cfg.Security.UploadDirs, cfg.Security.DownloadDirs)
	tasks := taskmanager.New()
	h := handler.New(cfg.AgentID, client, tasks, secValidator)
	h.Subscribe()
	h.StartHeartbeat(time.Duration(cfg.HeartbeatInterval) * time.Second)

	log.Println("Agent is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)

	h.StopHeartbeat()

	// Publish offline status before disconnecting
	statusTopic := fmt.Sprintf("clawpeteer/%s/status", cfg.AgentID)
	token = client.Publish(statusTopic, 1, true, `{"status":"offline"}`)
	token.Wait()

	client.Disconnect(1000)
	log.Println("Agent stopped.")
}

// loadConfig loads config with priority: file flag > embedded > defaults.
func loadConfig(configPath string) (*config.Config, error) {
	// Priority 1: explicit config file
	if configPath != "" {
		log.Printf("Loading config from file: %s", configPath)
		return config.Load(configPath)
	}

	// Priority 2: embedded config
	if data := buildcfg.LoadEmbeddedConfig(); data != nil {
		log.Println("Using embedded configuration")
		return config.Parse(data)
	}

	// Priority 3: default config.json in current directory
	if _, err := os.Stat("config.json"); err == nil {
		log.Println("Loading config from ./config.json")
		return config.Load("config.json")
	}

	// No config found — return defaults (CLI flags will fill in required fields)
	log.Println("No config file found, using defaults (provide --id and --broker)")
	return config.Defaults(), nil
}
