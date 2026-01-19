package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/spacemule/oauth2-proxy-injector/internal/admission"
	"github.com/spacemule/oauth2-proxy-injector/internal/annotation"
	"github.com/spacemule/oauth2-proxy-injector/internal/config"
	"github.com/spacemule/oauth2-proxy-injector/internal/mutation"
	"github.com/spacemule/oauth2-proxy-injector/internal/service"
)


type cmdConfig struct {
    port             int
    certFile         string
    keyFile          string
    configNamespace  string
    defaultConfigMap string
}

// main is the entrypoint for the webhook server
func main() {
	klog.InitFlags(nil)
	cfg := parseFlags()
	client, err := createKubernetesClient()
	if err != nil {
		klog.Fatal("failed to create kubernetes client: ", err)
	}
	parser := annotation.NewParser()
	loader := config.NewLoader(client, cfg.configNamespace)
	builder := mutation.NewSidecarBuilder()
	merger := config.NewMerger()
	knativeDetector := mutation.NewKnativeDetector()
	podMutator := mutation.NewPodMutator(parser, loader, builder, merger, knativeDetector, cfg.defaultConfigMap, cfg.configNamespace)
	podHandler := admission.NewHandler(podMutator)

	serviceMutator := service.NewServiceMutator()
	serviceHandler := service.NewHandler(serviceMutator)

	server, err := setupServer(podHandler, serviceHandler, client, cfg.certFile, cfg.keyFile, cfg.port)
	if err != nil {
		klog.Fatal("failed to create server: ", err)
	}

	go func() {
		klog.InfoS("starting server", "port", cfg.port)
		if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			klog.ErrorS(err, "server error")
			os.Exit(1)
		}
	}()

	gracefulShutdown(server)
}

// parseFlags parses command line flags and returns configuration
func parseFlags() cmdConfig {
	c := cmdConfig{}
	flag.IntVar(&c.port, "port", 8443, "HTTPS port to listen on")
	flag.StringVar(&c.certFile, "cert-file", "", "path to TLS certificate")
    flag.StringVar(&c.keyFile, "key-file", "", "path to TLS private key")
    flag.StringVar(&c.configNamespace, "config-namespace", "", "namespace for ConfigMaps")
	flag.StringVar(&c.defaultConfigMap, "default-config", "", "default configuration ConfigMap (optional)")

    flag.Parse()

    if c.certFile == "" || c.keyFile == "" {
        klog.Fatal("--cert-file and --key-file are required")
    }

    return c
}

// createKubernetesClient creates an in-cluster Kubernetes clientset
func createKubernetesClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// setupServer creates and configures the HTTPS server
//
// TODO: Update signature to accept both handlers
func setupServer(podHandler *admission.Handler, serviceHandler *service.Handler, client kubernetes.Interface, certFile, keyFile string, port int) (*http.Server, error) {
	m := http.NewServeMux()

	m.HandleFunc("/mutate", podHandler.HandleAdmission)
	// Alias for clarity
	m.HandleFunc("/mutate-pod", podHandler.HandleAdmission)

	m.HandleFunc("/mutate-service", serviceHandler.HandleAdmission)
	m.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	m.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		_, err := client.Discovery().ServerVersion()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	
	return &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: m,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}, nil
}

// gracefulShutdown handles SIGTERM/SIGINT for clean shutdown
func gracefulShutdown(server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit

	klog.Info("shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := server.Shutdown(ctx)
	if err != nil {
		klog.ErrorS(err, "server error")
	}

	klog.Info("server stopped")
}