package funcs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/snple/beacon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"
)

type Root struct {
	rootCmd *cobra.Command
	cfgFile string

	conn *grpc.ClientConn

	logger *zap.Logger
}

func NewRoot() *Root {
	r := &Root{
		rootCmd: &cobra.Command{
			Use:   "beacon",
			Short: "Beacon cli tool",
			Long:  `Beacon is a tool for managing your beacon chain`,
		},
	}

	r.init()

	return r
}

func (r *Root) Execute() error {
	return r.rootCmd.Execute()
}

func (r *Root) init() {
	cobra.OnInitialize(r.initConfig)

	r.rootCmd.PersistentFlags().StringVarP(&r.cfgFile, "config", "c", "beacon.toml", "config file")
	r.rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug")
	r.rootCmd.PersistentFlags().StringP("addr", "a", "127.0.0.1:6006", "addr")
	r.rootCmd.PersistentFlags().StringP("ca", "", "certs/ca.crt", "ca")
	r.rootCmd.PersistentFlags().StringP("cert", "", "certs/client.crt", "cert")
	r.rootCmd.PersistentFlags().StringP("key", "", "certs/client.key", "key")
	r.rootCmd.PersistentFlags().StringP("server_name", "", "beacon", "server name")
	r.rootCmd.PersistentFlags().BoolP("insecure_skip_verify", "", false, "insecure skip verify")

	r.rootCmd.AddCommand(r.versionCmd())
	r.rootCmd.AddCommand(r.nodeCmd())

	viper.BindPFlag("debug", r.rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("addr", r.rootCmd.PersistentFlags().Lookup("addr"))
	viper.BindPFlag("ca", r.rootCmd.PersistentFlags().Lookup("ca"))
	viper.BindPFlag("cert", r.rootCmd.PersistentFlags().Lookup("cert"))
	viper.BindPFlag("key", r.rootCmd.PersistentFlags().Lookup("key"))
	viper.BindPFlag("server_name", r.rootCmd.PersistentFlags().Lookup("server_name"))
	viper.BindPFlag("insecure_skip_verify", r.rootCmd.PersistentFlags().Lookup("insecure_skip_verify"))

	{
		var logger *zap.Logger
		var err error

		if viper.GetBool("debug") {
			logger, err = zap.NewDevelopment()
		} else {
			logger, err = zap.NewProduction()
		}

		if err != nil {
			log.Fatal(err)
		}
		defer logger.Sync()

		r.logger = logger
	}
}

func (r *Root) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Beacon",
		Long:  `All software has versions. This is Beacon's`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Beacon v%s\n", beacon.Version)
		},
	}
}

func (r *Root) initConfig() {
	if r.cfgFile != "" {
		viper.SetConfigFile(r.cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigType("toml")
		viper.SetConfigName("beacon")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		r.logger.Error("failed to read config file", zap.Error(err))
		os.Exit(1)
	}

	r.logger.Debug("Using config file", zap.String("file", viper.ConfigFileUsed()))
}

func (r *Root) GetConn() *grpc.ClientConn {
	if r.conn == nil {
		err := r.connect()
		if err != nil {
			r.logger.Error("failed to connect to server", zap.Error(err))
			os.Exit(1)
		}
	}

	return r.conn
}

func (r *Root) connect() error {
	if viper.GetBool("debug") {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stdout, os.Stdout))
	}

	var kacp = keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	cfg, err := loadCert()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(viper.GetString("addr"),
		grpc.WithTransportCredentials(credentials.NewTLS(cfg)),
		grpc.WithKeepaliveParams(kacp),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		s := conn.GetState()
		if s == connectivity.Idle {
			conn.Connect()
		}
		if s == connectivity.Ready {
			break
		}

		if !conn.WaitForStateChange(ctx, s) {
			return errors.New("failed to connect to server")
		}
	}

	r.conn = conn

	return nil
}

func (r *Root) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func loadCert() (*tls.Config, error) {
	pool := x509.NewCertPool()

	ca, err := os.ReadFile(viper.GetString("ca"))
	if err != nil {
		return nil, err
	}

	if ok := pool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("pool.AppendCertsFromPEM err")
	}

	cert, err := tls.LoadX509KeyPair(viper.GetString("cert"), viper.GetString("key"))
	if err != nil {
		return nil, err
	}

	cfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ServerName:         viper.GetString("server_name"),
		RootCAs:            pool,
		InsecureSkipVerify: viper.GetBool("insecure_skip_verify"),
	}

	return cfg, nil
}
