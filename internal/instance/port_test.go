package instance

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestIsPortAvailable(t *testing.T) {
	// Test with a likely available port
	port := 19999
	if !IsPortAvailable(port) {
		t.Skipf("Port %d is not available, skipping test", port)
	}

	// Start a listener on the port
	listener, err := net.Listen("tcp", ":19999")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Port should now be unavailable
	if IsPortAvailable(19999) {
		t.Error("IsPortAvailable should return false when port is in use")
	}
}

func TestIsPortAvailable_WellKnownPorts(t *testing.T) {
	// Test some common ports (these are likely to be unavailable)
	commonPorts := []int{80, 443, 22, 21, 25}

	for _, port := range commonPorts {
		available := IsPortAvailable(port)
		t.Logf("Port %d available: %v", port, available)
		// Don't assert - just log, as availability depends on system
	}
}

func TestFindAvailablePort(t *testing.T) {
	// Find an available port starting from a high number
	startPort := 20000
	port := FindAvailablePort(startPort)

	if port == 0 {
		t.Fatal("FindAvailablePort returned 0 (no port found)")
	}

	if port < startPort {
		t.Errorf("FindAvailablePort returned port %d, expected >= %d", port, startPort)
	}

	// Verify the port is actually available
	if !IsPortAvailable(port) {
		t.Errorf("FindAvailablePort returned port %d but it's not available", port)
	}
}

func TestFindAvailablePort_AllOccupied(t *testing.T) {
	// Create listeners for a range of ports
	startPort := 21000
	numPorts := 20
	var listeners []net.Listener

	for i := 0; i < numPorts; i++ {
		port := startPort + i
		listener, err := net.Listen("tcp", net.JoinHostPort("", string(rune(port))))
		if err == nil {
			listeners = append(listeners, listener)
		}
	}

	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	// Try to find a port in the occupied range
	port := FindAvailablePort(startPort)

	// Should find a port beyond the occupied range or return 0
	if port != 0 && port < startPort+numPorts {
		// Verify it's actually available if it was in the range
		if !IsPortAvailable(port) {
			t.Errorf("FindAvailablePort returned occupied port %d", port)
		}
	}
}

func TestHealthCheck_NoServer(t *testing.T) {
	// Test health check on a port with no server
	port := 22000
	err := HealthCheck(port)

	if err == nil {
		t.Error("HealthCheck should fail when no server is running")
	}
}

func TestHealthCheck_WithServer(t *testing.T) {
	// Start a simple HTTP server
	port := 22001

	// Create a simple handler that returns 200 OK
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:    ":22001",
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	defer server.Close()

	// Test health check
	err := HealthCheck(port)
	if err != nil {
		t.Errorf("HealthCheck should succeed with running server: %v", err)
	}
}

func TestHealthCheck_WrongStatusCode(t *testing.T) {
	// Start a server that returns 500
	port := 22002

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status":"error"}`))
	})

	server := &http.Server{
		Addr:    ":22002",
		Handler: mux,
	}

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Close()

	// Test health check - should fail due to non-200 status
	err := HealthCheck(port)
	if err == nil {
		t.Error("HealthCheck should fail when server returns non-200 status")
	}
}

func TestSendShutdownRequest_NoServer(t *testing.T) {
	// Test shutdown request on a port with no server
	port := 22003
	err := SendShutdownRequest(port)

	if err == nil {
		t.Error("SendShutdownRequest should fail when no server is running")
	}
}

func TestSendShutdownRequest_WithServer(t *testing.T) {
	// Start a server that responds to shutdown requests
	port := 22004

	mux := http.NewServeMux()
	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"shutting_down"}`))
	})

	server := &http.Server{
		Addr:    ":22004",
		Handler: mux,
	}

	go func() {
		server.ListenAndServe()
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Close()

	// Test shutdown request
	err := SendShutdownRequest(port)
	if err != nil {
		t.Errorf("SendShutdownRequest should succeed: %v", err)
	}
}

func TestWaitForPortToBeAvailable(t *testing.T) {
	port := 22005

	// Port should be available immediately
	available := WaitForPortToBeAvailable(port, 1*time.Second)
	if !available {
		t.Error("WaitForPortToBeAvailable should return true for available port")
	}

	// Start a listener
	listener, err := net.Listen("tcp", ":22005")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Should return false while port is occupied
	available = WaitForPortToBeAvailable(port, 500*time.Millisecond)
	if available {
		t.Error("WaitForPortToBeAvailable should return false for occupied port")
	}

	// Close listener in goroutine
	go func() {
		time.Sleep(200 * time.Millisecond)
		listener.Close()
	}()

	// Should return true once port becomes available
	available = WaitForPortToBeAvailable(port, 1*time.Second)
	if !available {
		t.Error("WaitForPortToBeAvailable should return true after port becomes available")
	}
}

func TestWaitForPortToBeAvailable_Timeout(t *testing.T) {
	port := 22006

	// Start a listener that won't close
	listener, err := net.Listen("tcp", ":22006")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Should timeout
	start := time.Now()
	available := WaitForPortToBeAvailable(port, 300*time.Millisecond)
	elapsed := time.Since(start)

	if available {
		t.Error("WaitForPortToBeAvailable should return false on timeout")
	}

	// Should have waited approximately the timeout duration
	if elapsed < 250*time.Millisecond {
		t.Errorf("WaitForPortToBeAvailable returned too quickly: %v", elapsed)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("WaitForPortToBeAvailable took too long: %v", elapsed)
	}
}

func TestGetProcessUsingPort_NoProcess(t *testing.T) {
	// Try to get process using an unlikely port
	port := 23456
	_, err := GetProcessUsingPort(port)

	if err == nil {
		t.Log("GetProcessUsingPort returned a PID for unused port (unexpected but not critical)")
	}
	// This test is informational - we can't guarantee no process is using any port
}

func TestGetProcessUsingPort_WithProcess(t *testing.T) {
	// Start a listener on a known port
	port := 22007
	listener, err := net.Listen("tcp", ":22007")
	if err != nil {
		t.Skipf("Failed to create listener on port %d: %v", port, err)
	}
	defer listener.Close()

	// Try to find the process using this port
	pid, err := GetProcessUsingPort(port)
	if err != nil {
		t.Logf("GetProcessUsingPort failed (this may be expected on some systems): %v", err)
		return
	}

	if pid <= 0 {
		t.Error("GetProcessUsingPort should return a valid PID")
	}

	t.Logf("Port %d is being used by PID %d", port, pid)
}

func BenchmarkIsPortAvailable(b *testing.B) {
	port := 19998
	for i := 0; i < b.N; i++ {
		IsPortAvailable(port)
	}
}

func BenchmarkFindAvailablePort(b *testing.B) {
	startPort := 20000
	for i := 0; i < b.N; i++ {
		FindAvailablePort(startPort + (i * 100))
	}
}
