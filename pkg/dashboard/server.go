package dashboard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/albertofilice/node-check-operator/pkg/dashboard/api"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DashboardServer represents the dashboard web server
type DashboardServer struct {
	server    *http.Server
	k8sClient client.Client
	clientset *kubernetes.Clientset
	namespace string
	port      int
}

// NewDashboardServer creates a new dashboard server
func NewDashboardServer(k8sClient client.Client, clientset *kubernetes.Clientset, namespace string, port int) *DashboardServer {
	return &DashboardServer{
		k8sClient: k8sClient,
		clientset: clientset,
		namespace: namespace,
		port:      port,
	}
}

// Start starts the dashboard server
func (ds *DashboardServer) Start() error {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.Default()

	// Setup CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Setup API routes
	dashboardAPI := api.NewDashboardAPI(ds.k8sClient, ds.clientset, ds.namespace)
	dashboardAPI.SetupRoutes(router)

	// Setup web routes
	ds.setupWebRoutes(router)

	// Create HTTP/HTTPS server
	ds.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", ds.port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// TLS certificates are REQUIRED - the server will only start with HTTPS
	// In OpenShift, the Service Serving Certificate Signer creates the secret automatically
	// but it may take a few seconds after the pod starts
	certPath := "/etc/tls/tls.crt"
	keyPath := "/etc/tls/tls.key"

	fmt.Printf("Dashboard server: Waiting for TLS certificates at %s and %s (required for HTTPS)\n", certPath, keyPath)
	fmt.Printf("Dashboard server: Note - Certificates are created by OpenShift Service Serving Certificate Signer\n")
	fmt.Printf("Dashboard server: The Service 'node-check-operator-dashboard' must exist with annotation 'service.beta.openshift.io/serving-cert-secret-name: node-check-operator-dashboard-tls'\n")
	
	// Wait for the certificate/key to be mounted and verify they are valid
	// Increase timeout to 10 minutes to give OpenShift more time to provision certificates
	// The Service Serving Certificate Signer may take time to create the secret
	if !waitForTLSCertificates(certPath, keyPath, 10*time.Minute) {
		fmt.Printf("Dashboard server: WARNING - TLS certificates not found after 10 minutes\n")
		fmt.Printf("Dashboard server: This may indicate:\n")
		fmt.Printf("  1. The Service 'node-check-operator-dashboard' does not exist\n")
		fmt.Printf("  2. The Service does not have the correct annotation\n")
		fmt.Printf("  3. OpenShift Service Serving Certificate Signer is not working\n")
		fmt.Printf("  4. The secret 'node-check-operator-dashboard-tls' was not created\n")
		fmt.Printf("Dashboard server: Check the Service and secret in namespace '%s'\n", ds.namespace)
		return fmt.Errorf("TLS certificates not found after 10 minutes - dashboard server requires HTTPS and cannot start without certificates")
	}

	// Verify certificates are valid before using them
	if !verifyTLSCertificates(certPath, keyPath) {
		return fmt.Errorf("TLS certificates found but invalid - dashboard server requires valid certificates to start")
	}

	// Configure TLS
	ds.server.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Start HTTPS server
	go func() {
		if err := ds.server.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Dashboard server error (HTTPS): %v\n", err)
		}
	}()
	fmt.Printf("Dashboard server started with HTTPS on port %d (TLS certificates verified)\n", ds.port)

	return nil
}

// Stop stops the dashboard server
func (ds *DashboardServer) Stop(ctx context.Context) error {
	if ds.server != nil {
		return ds.server.Shutdown(ctx)
	}
	return nil
}

// setupWebRoutes sets up the web routes
func (ds *DashboardServer) setupWebRoutes(router *gin.Engine) {
	// Main dashboard page
	router.GET("/", ds.dashboardPage)
	router.GET("/dashboard", ds.dashboardPage)

	// Node detail page
	router.GET("/node/:name", ds.nodeDetailPage)

	// NodeCheck detail page
	router.GET("/nodecheck/:name", ds.nodeCheckDetailPage)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
}

// dashboardPage renders the main dashboard page
func (ds *DashboardServer) dashboardPage(c *gin.Context) {
	// Return JSON response instead of HTML template
	c.JSON(http.StatusOK, gin.H{
		"message": "Node Check Dashboard API",
		"endpoints": gin.H{
			"stats":      "/api/v1/stats",
			"nodechecks": "/api/v1/nodechecks",
			"health":     "/health",
		},
	})
}

// nodeDetailPage renders the node detail page
func (ds *DashboardServer) nodeDetailPage(c *gin.Context) {
	nodeName := c.Param("name")
	// Return JSON response instead of HTML template
	c.JSON(http.StatusOK, gin.H{
		"message":  "Node detail endpoint",
		"nodeName": nodeName,
		"endpoint": fmt.Sprintf("/api/v1/nodes/%s", nodeName),
	})
}

// nodeCheckDetailPage renders the NodeCheck detail page
func (ds *DashboardServer) nodeCheckDetailPage(c *gin.Context) {
	nodeCheckName := c.Param("name")
	// Return JSON response instead of HTML template
	c.JSON(http.StatusOK, gin.H{
		"message":       "NodeCheck detail endpoint",
		"nodeCheckName": nodeCheckName,
		"endpoint":      fmt.Sprintf("/api/v1/nodechecks/%s", nodeCheckName),
	})
}

// waitForTLSCertificates waits for the TLS certificate/key files to appear within the given timeout.
func waitForTLSCertificates(certPath, keyPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	checkInterval := 2 * time.Second
	attempts := 0
	
	for time.Now().Before(deadline) {
		attempts++
		certExists := false
		keyExists := false
		
		if _, err := os.Stat(certPath); err == nil {
			certExists = true
		}
		if _, err := os.Stat(keyPath); err == nil {
			keyExists = true
		}
		
		if certExists && keyExists {
			fmt.Printf("Dashboard server: TLS certificates found after %d attempts\n", attempts)
			return true
		}
		
		if attempts%15 == 0 { // Log every 30 seconds
			fmt.Printf("Dashboard server: Still waiting for TLS certificates (attempt %d, cert: %v, key: %v)\n", 
				attempts, certExists, keyExists)
			// Check if the directory exists
			if dirInfo, err := os.Stat("/etc/tls"); err == nil {
				fmt.Printf("Dashboard server: /etc/tls directory exists (isDir: %v)\n", dirInfo.IsDir())
			} else {
				fmt.Printf("Dashboard server: /etc/tls directory does not exist: %v\n", err)
			}
		}
		
		time.Sleep(checkInterval)
	}
	
	fmt.Printf("Dashboard server: TLS certificates not found after %d attempts (timeout: %v)\n", attempts, timeout)
	return false
}

// verifyTLSCertificates verifies that the TLS certificate and key files are valid and can be loaded.
func verifyTLSCertificates(certPath, keyPath string) bool {
	// Try to load the certificate and key to verify they are valid
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		fmt.Printf("Dashboard server: Failed to load TLS certificates: %v\n", err)
		return false
	}
	
	if len(cert.Certificate) == 0 {
		fmt.Printf("Dashboard server: TLS certificate is empty\n")
		return false
	}
	
	fmt.Printf("Dashboard server: TLS certificates verified successfully\n")
	return true
}
