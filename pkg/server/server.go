// Package server provides a built-in HTTP server for serving static files.
package server

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Server represents a built-in HTTP file server.
type Server struct {
	server      *http.Server
	port        int
	directory   string
	listener    net.Listener
	done        chan struct{}
	uploadPath  string
	spaMode     bool // Serve index.html for all routes (SPA support)
	showListing bool // Show directory listing if no index.html
}

// Config holds the server configuration.
type Config struct {
	Port         int
	Directory    string
	EnableUpload bool
	SPAMode      bool // Enable SPA mode (fallback to index.html)
	ShowListing  bool // Show directory listing (default: false, serve index.html)
}

// FileInfo represents a file in directory listing.
type FileInfo struct {
	Name    string
	Size    string
	ModTime string
	IsDir   bool
	Path    string
}

// New creates a new HTTP file server.
func New(cfg Config) (*Server, error) {
	// Resolve directory path
	dir := cfg.Directory
	if dir == "" {
		dir = "."
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absDir)
	}

	// Find available port
	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// Try to find an available port
		listener, err = net.Listen("tcp", ":0")
		if err != nil {
			return nil, fmt.Errorf("failed to find available port: %w", err)
		}
		port = listener.Addr().(*net.TCPAddr).Port
	}

	s := &Server{
		port:        port,
		directory:   absDir,
		listener:    listener,
		done:        make(chan struct{}),
		spaMode:     cfg.SPAMode,
		showListing: cfg.ShowListing,
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
		close(s.done)
	}()
	return nil
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// Directory returns the directory being served.
func (s *Server) Directory() string {
	return s.directory
}

// Stop gracefully stops the server.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("server shutdown timeout")
	}
}

// Wait blocks until the server is stopped.
func (s *Server) Wait() {
	<-s.done
}

// handleRequest handles all incoming HTTP requests.
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Clean the path to prevent directory traversal
	urlPath := filepath.Clean(r.URL.Path)
	if urlPath == "" {
		urlPath = "/"
	}

	// Build the full file path
	filePath := filepath.Join(s.directory, urlPath)

	// Ensure the path is within the served directory
	if !strings.HasPrefix(filePath, s.directory) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if the file exists
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// File doesn't exist - check SPA mode
		if s.spaMode {
			// Serve index.html for SPA routing
			indexPath := filepath.Join(s.directory, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
		}
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Handle directories
	if info.IsDir() {
		// Try to serve index.html first
		indexPath := filepath.Join(filePath, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}

		// If no index.html and listing is enabled, show directory listing
		if s.showListing {
			s.serveDirectory(w, r, filePath, urlPath)
			return
		}

		// Otherwise, return 404 or forbidden
		http.NotFound(w, r)
		return
	}

	// Serve the file
	http.ServeFile(w, r, filePath)
}

// serveDirectory renders a directory listing.
func (s *Server) serveDirectory(w http.ResponseWriter, r *http.Request, dirPath, urlPath string) {
	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	// Build file list
	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Skip hidden files (starting with .)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fi := FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Format("Jan 02, 2006 15:04"),
		}

		if entry.IsDir() {
			fi.Name += "/"
			fi.Size = "-"
			fi.Path = filepath.Join(urlPath, entry.Name()) + "/"
		} else {
			fi.Size = formatFileSize(info.Size())
			fi.Path = filepath.Join(urlPath, entry.Name())
		}

		files = append(files, fi)
	}

	// Sort: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Add parent directory link if not at root
	if urlPath != "/" {
		parent := filepath.Dir(urlPath)
		if parent == "." {
			parent = "/"
		}
		files = append([]FileInfo{{
			Name:  "../",
			IsDir: true,
			Size:  "-",
			Path:  parent,
		}}, files...)
	}

	// Render template
	data := struct {
		Title     string
		Path      string
		Files     []FileInfo
		Directory string
	}{
		Title:     filepath.Base(dirPath),
		Path:      urlPath,
		Files:     files,
		Directory: dirPath,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := directoryTemplate.Execute(w, data); err != nil {
		http.Error(w, "Failed to render directory listing", http.StatusInternalServerError)
	}
}

// formatFileSize formats a file size in bytes to a human-readable string.
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// walkDir recursively walks a directory and returns file info.
func walkDir(root string) ([]fs.FileInfo, error) {
	var files []fs.FileInfo
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if !info.IsDir() {
			files = append(files, info)
		}
		return nil
	})
	return files, err
}

// Directory listing HTML template
var directoryTemplate = template.Must(template.New("directory").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} - qrlocal</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px 24px;
        }
        header h1 {
            font-size: 1.5rem;
            font-weight: 600;
        }
        header .path {
            font-size: 0.9rem;
            opacity: 0.9;
            margin-top: 4px;
            word-break: break-all;
        }
        .file-list {
            list-style: none;
        }
        .file-list li {
            display: flex;
            align-items: center;
            padding: 12px 24px;
            border-bottom: 1px solid #eee;
            transition: background 0.2s;
        }
        .file-list li:hover {
            background: #f9f9f9;
        }
        .file-list li:last-child {
            border-bottom: none;
        }
        .file-list a {
            color: #333;
            text-decoration: none;
            flex: 1;
            display: flex;
            align-items: center;
        }
        .file-list a:hover {
            color: #667eea;
        }
        .icon {
            width: 24px;
            height: 24px;
            margin-right: 12px;
            flex-shrink: 0;
        }
        .icon-folder {
            color: #ffc107;
        }
        .icon-file {
            color: #6c757d;
        }
        .name {
            flex: 1;
            word-break: break-word;
        }
        .meta {
            display: flex;
            gap: 24px;
            color: #888;
            font-size: 0.85rem;
        }
        .size {
            min-width: 80px;
            text-align: right;
        }
        .date {
            min-width: 140px;
            text-align: right;
        }
        footer {
            padding: 16px 24px;
            background: #f9f9f9;
            color: #888;
            font-size: 0.85rem;
            text-align: center;
        }
        footer a {
            color: #667eea;
            text-decoration: none;
        }
        @media (max-width: 600px) {
            body {
                padding: 10px;
            }
            .file-list li {
                padding: 10px 16px;
            }
            .meta {
                flex-direction: column;
                gap: 4px;
                align-items: flex-end;
            }
            .date {
                display: none;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>📁 {{.Title}}</h1>
            <div class="path">{{.Path}}</div>
        </header>
        <ul class="file-list">
            {{range .Files}}
            <li>
                <a href="{{.Path}}">
                    {{if .IsDir}}
                    <svg class="icon icon-folder" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>
                    </svg>
                    {{else}}
                    <svg class="icon icon-file" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm4 18H6V4h7v5h5v11z"/>
                    </svg>
                    {{end}}
                    <span class="name">{{.Name}}</span>
                </a>
                <div class="meta">
                    <span class="size">{{.Size}}</span>
                    <span class="date">{{.ModTime}}</span>
                </div>
            </li>
            {{end}}
        </ul>
        <footer>
            Served by <a href="https://github.com/dendysatrya/qrlocal">qrlocal</a>
        </footer>
    </div>
</body>
</html>
`))
