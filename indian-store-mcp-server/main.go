package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"indian-store-mcp-server/internal/config"
	"indian-store-mcp-server/internal/middleware"
	"indian-store-mcp-server/internal/oauth"
)

// JSON-RPC 2.0 structures (matching the mcp-service pattern)
type JSONRPCRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Protocol structures
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     map[string]interface{} `json:"sampling,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools        *ToolsCapability       `json:"tools,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type MCPServer struct {
	initialized bool
	mu          sync.RWMutex
}

func NewMCPServer() *MCPServer {
	return &MCPServer{}
}

func (s *MCPServer) handleRequest(req JSONRPCRequest) JSONRPCResponse {
	log.Printf("Received request: method=%s id=%v", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID, req.Params)
	case "notifications/initialized":
		log.Println("Client initialized notification received")
		return JSONRPCResponse{} // No response for notifications
	case "tools/list":
		s.mu.RLock()
		initialized := s.initialized
		s.mu.RUnlock()
		if !initialized {
			return s.sendError(req.ID, -32002, "Server not initialized", nil)
		}
		return s.handleToolsList(req.ID)
	case "tools/call":
		s.mu.RLock()
		initialized := s.initialized
		s.mu.RUnlock()
		if !initialized {
			return s.sendError(req.ID, -32002, "Server not initialized", nil)
		}
		return s.handleCallTool(req.ID, req.Params)
	case "ping":
		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{},
		}
	default:
		return s.sendError(req.ID, -32601, "Method not found", req.Method)
	}
}

func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *MCPServer) handleInitialize(id interface{}, params json.RawMessage) JSONRPCResponse {
	var initParams InitializeParams
	if err := json.Unmarshal(params, &initParams); err != nil {
		return s.sendError(id, -32602, "Invalid params", err.Error())
	}

	log.Printf("Initialize request from client: %s %s", initParams.ClientInfo.Name, initParams.ClientInfo.Version)

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",  // Match the mcp-service version
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false, // No dynamic tool list changes
			},
		},
		ServerInfo: ServerInfo{
			Name:    "indian-store-mcp-server",
			Version: "1.0.0",
		},
	}

	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *MCPServer) handleToolsList(id interface{}) JSONRPCResponse {
	tools := []Tool{
		{
			Name:        "list_indian_stores",
			Description: "List popular Indian online stores with their services",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
	}

	result := ToolsListResult{Tools: tools}

	return JSONRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (s *MCPServer) handleCallTool(id interface{}, params json.RawMessage) JSONRPCResponse {
	var callParams CallToolParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return s.sendError(id, -32602, "Invalid params", err.Error())
	}

	log.Printf("Tool call: %s with args: %v", callParams.Name, callParams.Arguments)

	switch callParams.Name {
	case "list_indian_stores":
		return JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []Content{
					{
						Type: "text",
						Text: "1. Flipkart - E-commerce platform offering electronics, fashion, home essentials\n2. Amazon India - Global e-commerce platform with wide product range\n3. Reliance Digital - Electronics and appliances retailer\n4. Myntra - Fashion and lifestyle e-commerce platform\n5. Snapdeal - E-commerce platform with various product categories\n6. Tata CLiQ - Digital commerce platform by Tata Group",
					},
				},
			},
		}
	default:
		return s.sendError(id, -32601, "Unknown tool", callParams.Name)
	}
}

// HTTP handler for MCP requests
func (s *MCPServer) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	// Set appropriate headers for MCP communication
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Invalid JSON-RPC request: %v", err)

		// Send back a parse error response
		errorResponse := JSONRPCResponse{
			JsonRPC: "2.0",
			ID:      nil, // Parse errors typically have ID as null
			Error: &RPCError{
				Code:    -32700, // Parse error
				Message: "Parse error: Invalid JSON",
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// Process the request
	response := s.handleRequest(req)

	// Send response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"server": "indian-store-mcp-server",
	})
}

// OAuth Discovery endpoint for MCP clients
func oauthDiscovery(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Determine the base URL from headers or construct from request
		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "" {
			scheme = "http"
		}
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
		
		host := r.Host
		if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}
		
		baseURL := scheme + "://" + host

		// OAuth discovery - clients use Ory Hydra directly, we only provide registration
		discovery := map[string]interface{}{
			"issuer":                                baseURL,
			"authorization_endpoint":                baseURL + "/oauth2/auth",
			"token_endpoint":                        baseURL + "/oauth2/token",
			"registration_endpoint":                 baseURL + "/oauth/register",
			"userinfo_endpoint":                     baseURL + "/oauth2/userinfo",
			"introspection_endpoint":                baseURL + "/oauth2/introspect",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
			"scopes_supported":                      []string{"openid", "offline_access", "email", "profile"},
			"subject_types_supported":               []string{"public"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(discovery)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load configuration
	cfg := config.Load()
	log.Println("Configuration loaded successfully")

	// Initialize Ory client
	oryClient := oauth.NewOryClient(cfg)
	log.Printf("Ory client initialized with URL: %s", cfg.OryURL)

	// Create registration handler for dynamic client registration
	registrationHandler := oauth.NewRegistrationHandler(cfg, oryClient)
	
	// Create login/consent handler for Ory Hydra flows
	loginConsentHandler := oauth.NewLoginConsentHandler(oryClient)

	// Create authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(oryClient)

	// Create MCP server
	server := NewMCPServer()

	// OAuth discovery endpoint (required by MCP clients)
	http.HandleFunc("/.well-known/oauth-authorization-server", middleware.CORS(oauthDiscovery(cfg)))

	// Setup OAuth registration endpoint (only endpoint we handle, rest is Ory)
	http.HandleFunc("/oauth/register", middleware.CORS(registrationHandler.HandleRegister))
	
	// Redirect /oauth/authorize to /oauth2/auth for backward compatibility with cached clients
	http.HandleFunc("/oauth/authorize", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {
		// Simply redirect to Ory's authorization endpoint with same query params
		newURL := "https://" + r.Host + "/oauth2/auth?" + r.URL.RawQuery
		http.Redirect(w, r, newURL, http.StatusFound)
	}))

	// Hydra Login/Consent and error fallback pages
	http.HandleFunc("/login", middleware.CORS(loginConsentHandler.HandleLogin))
	http.HandleFunc("/consent", middleware.CORS(loginConsentHandler.HandleConsent))
	http.HandleFunc("/oauth2/fallbacks/error", middleware.CORS(loginConsentHandler.HandleError))

	// Setup MCP endpoint (protected with auth)
	http.HandleFunc("/mcp", middleware.CORS(authMiddleware.RequireAuth(server.handleMCPRequest)))

	// Health check (no auth required)
	http.HandleFunc("/health", healthCheck)

	// Start server
	addr := cfg.Host + ":" + cfg.Port
	log.Printf("Indian Store MCP Server with Ory OAuth starting on %s", addr)
	log.Printf("OAuth Authorize: /oauth/authorize")
	log.Printf("OAuth Callback: /oauth/callback")
	log.Printf("MCP Endpoint (protected): /mcp")
	log.Fatal(http.ListenAndServe(addr, nil))
}
