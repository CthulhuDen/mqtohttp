package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := cli.Command{
		Name:    "mqtohttp",
		Version: "v0.1.0",
		Usage:   "Helper to translate MQTT messages into HTTP requests",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "mqtt-endpoint",
				Usage:    "MQTT server endpoint inc. protocol, but not credentials",
				Sources:  cli.EnvVars("MQTT_ENDPOINT"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "mqtt-user",
				Usage:   "Username for MQTT authentication",
				Sources: cli.EnvVars("MQTT_USER"),
			},
			&cli.StringFlag{
				Name:    "mqtt-password",
				Usage:   "Password for MQTT authentication",
				Sources: cli.EnvVars("MQTT_PASSWORD"),
			},
			&cli.StringFlag{
				Name:      "mqtt-session-file",
				Usage:     "File storing session ID for MQTT. Unless this file is persisted, messages arriving while we are disconnected from the MQTT server will be lost",
				Sources:   cli.EnvVars("MQTT_SESSION_FILE"),
				Value:     "session-id.txt",
				TakesFile: true,
			},
			&cli.Uint16Flag{
				Name:    "mqtt-keepalive",
				Usage:   "Keepalive period for MQTT authentication (interval for keepalive messages)",
				Value:   20,
				Sources: cli.EnvVars("MQTT_KEEPALIVE"),
			},
			&cli.Uint32Flag{
				Name:    "mqtt-session-expiry",
				Usage:   "Expiry in seconds for MQTT session. Determines how long can client be offline and then receive the missed messages. Recommended to use big enough values for production not to lose messages",
				Value:   uint32((7 * 24 * time.Hour).Seconds()),
				Sources: cli.EnvVars("MQTT_SESSION_EXPIRY"),
			},
			&cli.StringSliceFlag{
				Name:     "mqtt-topics",
				Aliases:  []string{"mt"},
				Usage:    "MQTT topics to subscribe to, can be specified multiple times",
				Sources:  cli.EnvVars("MQTT_TOPICS"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "http-endpoint",
				Aliases:  []string{"he"},
				Usage:    "HTTP server endpoint",
				Sources:  cli.EnvVars("HTTP_ENDPOINT"),
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			clientId, err := getClientId(cmd.String("mqtt-session-file"))
			if err != nil {
				return fmt.Errorf("getting MQTT client id: %w", err)
			}

			u, err := url.Parse(cmd.String("mqtt-endpoint"))
			if err != nil {
				return fmt.Errorf("parsing MQTT connect url: %w", err)
			}

			httpEndpoint := cmd.String("http-endpoint")

			// App will run until cancelled by user (e.g. ctrl-c)
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			ctx, cancel := context.WithCancelCause(ctx)

			cliCfg := autopaho.ClientConfig{
				ServerUrls:            []*url.URL{u},
				ConnectUsername:       cmd.String("mqtt-user"),
				ConnectPassword:       []byte(cmd.String("mqtt-password")),
				KeepAlive:             cmd.Uint16("mqtt-keepalive"),
				SessionExpiryInterval: cmd.Uint32("mqtt-session-expiry"),
				OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
					log.Println("mqtt connection up")

					topics := cmd.StringSlice("mqtt-topics")

					subs := make([]paho.SubscribeOptions, 0, len(topics))
					for _, t := range topics {
						subs = append(subs, paho.SubscribeOptions{
							Topic:          t,
							QoS:            2,
							RetainHandling: 2, // No retained messages
						})
					}

					_, err := cm.Subscribe(context.Background(), &paho.Subscribe{Subscriptions: subs})
					if err == nil {
						log.Printf("mqtt subscription ok\n")
					} else {
						cancel(fmt.Errorf("failed to subscribe to topics: %w", err))
					}
				},
				OnConnectError: func(err error) {
					log.Printf("error whilst attempting connection: %s. Will retry\n", err)
				},
				ClientConfig: paho.ClientConfig{
					ClientID:                   clientId,
					EnableManualAcknowledgment: true,
					OnPublishReceived: []func(paho.PublishReceived) (bool, error){
						func(pr paho.PublishReceived) (bool, error) {
							res, err := handleMessage(ctx, pr.Packet, httpEndpoint)
							if err == nil {
								log.Printf("handled message on %s: %s\n", pr.Packet.Topic, res)

								err := pr.Client.Ack(pr.Packet)
								if err != nil {
									log.Printf("error acking message on %s: %s\n", pr.Packet.Topic, err)
								}

								return true, nil
							}

							// problem: when we reject message (do not Ack it), we will NOT receive any further messages
							// so let's just shutdown and hope restart fixes whatever our problem is
							cancel(fmt.Errorf("failed to handle message on %s: %w", pr.Packet.Topic, err))

							return false, err
						}},
					OnClientError: func(err error) {
						cancel(fmt.Errorf("client error: %w", err))
					},
					OnServerDisconnect: func(d *paho.Disconnect) {
						if d.Properties != nil {
							log.Printf("server requested disconnect: %s\n", d.Properties.ReasonString)
						} else {
							log.Printf("server requested disconnect; reason code: %d\n", d.ReasonCode)
						}
					},
				},
			}

			c, err := autopaho.NewConnection(ctx, cliCfg) // starts process; will reconnect until context cancelled
			if err != nil {
				return fmt.Errorf("creating MQTT connection: %w", err)
			}

			select {
			case <-ctx.Done():
				log.Println("shutting down MQTT client")
				<-c.Done() // Wait for clean shutdown (cancelling the context triggered the shutdown)
			case <-c.Done():
			}

			log.Println("MQTT client is shut down")

			if err := context.Cause(ctx); !errors.Is(err, context.Canceled) {
				return err
			}

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func handleMessage(ctx context.Context, p *paho.Publish, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(p.Payload)))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("sent to %s (response code %d)", req.URL, resp.StatusCode), nil
}

func getClientId(file string) (string, error) {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		bs, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}

		if len(bs) == 0 {
			return "", fmt.Errorf("file contains no content, if this state persists - remove manually")
		}

		return string(bs), nil
	}

	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}

	defer f.Close()

	id := rand.Text()

	n, err := f.Write([]byte(id))
	if err == nil && n != len(id) {
		err = io.ErrShortWrite
	}

	return id, err
}
