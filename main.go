package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

// Config holds the application configuration.
type Config struct {
	DirpathToServe string
	ListenIp       string
	ListenPort     int
	LogLevel       string
}

// FileViewData holds information for displaying a file in the template.
type FileViewData struct {
	Name    string
	SizeMB  string
	ModTime string
}

// C is the global configuration variable.
var C Config

var (
	version = "dev" // is set during build time
)

// LogHook is a custom logrus hook to write to multiple outputs with different formatters.
type LogHook struct {
	writers    []io.Writer
	formatters []log.Formatter
	logLevels  []log.Level
}

// NewLogHook creates a new hook.
func (hook *LogHook) Add(writer io.Writer, formatter log.Formatter, levels []log.Level) {
	hook.writers = append(hook.writers, writer)
	hook.formatters = append(hook.formatters, formatter)
	hook.logLevels = append(hook.logLevels, levels...)
}

// Fire is called by logrus when a log entry is made.
func (hook *LogHook) Fire(entry *log.Entry) error {
	for i, writer := range hook.writers {
		// Check if the level of the entry is one of the levels for this writer
		isLoggable := false
		for _, level := range hook.logLevels {
			if entry.Level <= level {
				isLoggable = true
				break
			}
		}

		if isLoggable {
			formatted, err := hook.formatters[i].Format(entry)
			if err != nil {
				return err
			}
			if _, err := writer.Write(formatted); err != nil {
				return err
			}
		}
	}
	return nil
}

// Levels returns the levels that this hook is interested in.
func (hook *LogHook) Levels() []log.Level {
	return log.AllLevels
}

func setupLogging(level string) {
	spew.Config.Indent = "  "

	logLevel, err := log.ParseLevel(level)
	if err != nil {
		log.Fatalf("Invalid log level: %v", err)
	}
	log.SetLevel(logLevel)
	log.SetOutput(io.Discard) // All output is now handled by the hook

	logFile, err := os.OpenFile("/tmp/hfs.last.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	var consoleFormatter log.Formatter
	if isatty.IsTerminal(os.Stdout.Fd()) {
		consoleFormatter = &log.TextFormatter{ForceColors: true, FullTimestamp: true}
	} else {
		consoleFormatter = &log.JSONFormatter{}
	}

	hook := &LogHook{}
	hook.Add(os.Stdout, consoleFormatter, log.AllLevels)
	hook.Add(logFile, &log.JSONFormatter{}, log.AllLevels)
	log.AddHook(hook)
}

func main() {
	app := &cli.App{
		Name:    "http-file-server",
		Usage:   "A simple HTTP server for file listing, uploading, and downloading.",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "log-level", Value: "info", Usage: "Set log level (trace, debug, info, warn, error, fatal, panic)"},
			&cli.StringFlag{Name: "dir-to-serve", Aliases: []string{"d"}, Value: ".", Usage: "Directory to serve files from"},
			&cli.StringFlag{Name: "listen-ip", Value: "0.0.0.0", Usage: "IP address to listen on"},
			&cli.IntFlag{Name: "listen-port", Value: 8080, Usage: "Port to listen on"},
		},
		Before: func(c *cli.Context) error {
			C = Config{
				DirpathToServe: c.String("dir-to-serve"),
				ListenIp:       c.String("listen-ip"),
				ListenPort:     c.Int("listen-port"),
				LogLevel:       c.String("log-level"),
			}

			// Re-setup logging with the potentially new level.
			setupLogging(C.LogLevel)

			// Show user the effective config in use
			log.Info("Current configuration:")
			spew.Dump(C)

			return nil
		},
		Action: func(c *cli.Context) error {
			// Do not run server if a subcommand was called
			if c.NArg() > 0 && c.Command.Name != "" {
				return nil
			}
			return startServer()
		},
	}
	app.UseShortOptionHandling = true
	app.EnableBashCompletion = true

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func startServer() error {
	addr := fmt.Sprintf("%s:%d", C.ListenIp, C.ListenPort)
	log.Infof("Starting server on %s", addr)
	absPath, err := filepath.Abs(C.DirpathToServe)
	if err != nil {
		log.Errorf("Could not determine absolute path for %s: %v", C.DirpathToServe, err)
	} else {
		log.Infof("Serving files from: %s", absPath)
	}

	http.HandleFunc("/", listFilesHandler)
	http.HandleFunc("/upload", uploadFileHandler)
	http.HandleFunc("/delete", deleteFileHandler)
	http.HandleFunc("/download/", downloadFileHandler) // Add a dedicated handler for downloads
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(C.DirpathToServe))))

	return http.ListenAndServe(addr, nil)
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	dirEntries, err := os.ReadDir(C.DirpathToServe)
	if err != nil {
		log.Errorf("Failed to read directory %s: %v", C.DirpathToServe, err)
		http.Error(w, "Could not read directory", http.StatusInternalServerError)
		return
	}

	var files []FileViewData
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				log.Warnf("Could not get file info for %s: %v", entry.Name(), err)
				continue
			}
			files = append(files, FileViewData{
				Name:    entry.Name(),
				SizeMB:  fmt.Sprintf("%.2f MB", float64(info.Size())/(1024*1024)),
				ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}

	data := struct {
		Files []FileViewData
	}{
		Files: files,
	}

	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		log.Errorf("Failed to parse template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Errorf("Failed to execute template: %v", err)
	}
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Don't call ParseMultipartForm as it consumes the body
	// which prevents us from using MultipartReader

	// Get a multipart reader to process files as streams
	mr, err := r.MultipartReader()
	if err != nil {
		log.Errorf("Failed to get multipart reader: %v", err)
		http.Error(w, "Could not process upload", http.StatusInternalServerError)
		return
	}

	filesUploaded := 0

	// Process each part (file) in the multipart form
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break // No more parts
		}
		if err != nil {
			log.Errorf("Error reading next part: %v", err)
			http.Error(w, "Error processing upload", http.StatusInternalServerError)
			return
		}

		// Skip non-file parts
		if part.FileName() == "" {
			continue
		}

		// Get the filename from the part
		filename := filepath.Base(part.FileName())
		fileSize := int64(0) // Will track the file size

		log.Infof("Starting upload of file: %s", filename)

		// Create the destination file
		dstPath := filepath.Join(C.DirpathToServe, filename)
		dst, err := os.Create(dstPath)
		if err != nil {
			log.Errorf("Could not create file %s on server: %v", dstPath, err)
			http.Error(w, "Could not create file on server", http.StatusInternalServerError)
			return
		}

		// Copy from the part directly to the file on disk
		fileSize, err = io.Copy(dst, part)
		dst.Close() // Close file immediately after copying

		if err != nil {
			log.Errorf("Could not save file %s: %v", dstPath, err)
			// Try to remove the potentially partial file
			os.Remove(dstPath)
			http.Error(w, "Could not save file", http.StatusInternalServerError)
			return
		}

		log.Infof("Completed upload of file: %s (size: %d bytes)", filename, fileSize)
		filesUploaded++
	}

	log.Infof("Successfully uploaded %d files", filesUploaded)

	w.Header().Set("HX-Refresh", "true")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Errorf("Could not parse form for delete: %v", err)
		http.Error(w, "Could not parse form", http.StatusBadRequest)
		return
	}

	filesToDelete := r.Form["files"]
	for _, filename := range filesToDelete {
		// Basic security check to prevent path traversal
		if strings.Contains(filename, "..") {
			log.Warnf("Attempted path traversal on delete: %s", filename)
			continue
		}
		filePath := filepath.Join(C.DirpathToServe, filename)
		log.Infof("Deleting file: %s", filePath)
		if err := os.Remove(filePath); err != nil {
			log.Errorf("Failed to delete file %s: %v", filePath, err)
			// Continue to next file, don't stop the whole process
		}
	}

	w.Header().Set("HX-Refresh", "true")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// downloadFileHandler handles direct file downloads with proper headers for filenames with spaces
func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the filename from the URL path
	filename := strings.TrimPrefix(r.URL.Path, "/download/")

	// Basic security check to prevent path traversal
	if strings.Contains(filename, "..") {
		log.Warnf("Attempted path traversal: %s", filename)
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Construct the file path
	filePath := filepath.Join(C.DirpathToServe, filename)

	// Check if the file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warnf("File not found: %s", filePath)
			http.NotFound(w, r)
		} else {
			log.Errorf("Error accessing file %s: %v", filePath, err)
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	// Check if it's actually a file
	if fileInfo.IsDir() {
		log.Warnf("Requested path is a directory: %s", filePath)
		http.Error(w, "Cannot download a directory", http.StatusBadRequest)
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("Error opening file %s: %v", filePath, err)
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set the content disposition header to handle files with spaces properly
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Stream the file to the response
	_, err = io.Copy(w, file)
	if err != nil {
		log.Errorf("Error streaming file %s: %v", filePath, err)
	}
}

const indexHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>File Server</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        body { font-family: sans-serif; }
        .container { max-width: 800px; margin: auto; padding: 20px; }
        .file-list { list-style-type: none; padding: 0; }
        .file-item { display: flex; align-items: center; margin-bottom: 5px; }
        .file-item input { margin-right: 10px; }
        .file-item a { flex-grow: 1; }
        .actions { margin-top: 20px; }
        .upload-form { margin-top: 20px; border-top: 1px solid #ccc; padding-top: 20px; }
        progress { width: 100%; }
        .download-link { color: #0066cc; text-decoration: underline; cursor: pointer; }
        .custom-file-upload { 
            display: inline-block; 
            padding: 6px 12px; 
            cursor: pointer; 
            background-color: #f8f8f8; 
            border: 1px solid #ccc; 
            border-radius: 4px;
        }
        .file-input { 
            display: none; 
        }
        .download-notification {
            position: fixed;
            bottom: 20px;
            right: 20px;
            background-color: #4CAF50;
            color: white;
            padding: 15px;
            border-radius: 5px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.2);
            display: none;
            z-index: 1000;
            animation: fadeOut 3s forwards;
            animation-delay: 2s;
        }
        @keyframes fadeOut {
            from { opacity: 1; }
            to { opacity: 0; }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Files</h1>
        <form>
            <ul class="file-list">
                {{range .Files}}
                <li class="file-item">
                    <input type="checkbox" name="files" value="{{.Name}}">
                    <a href="/download/{{.Name}}" class="download-link" hx-boost="false" onclick="showDownloadStarted('{{.Name}}')">{{.Name}}</a>
                    <span style="padding-left: 1em; color: #555; white-space: nowrap;">{{.SizeMB}} &nbsp; {{.ModTime}}</span>
                </li>
                {{else}}
                <li>No files found.</li>
                {{end}}
            </ul>
            <div class="actions">
                <button type="button" hx-post="/delete" hx-target="body" hx-include="[name='files']:checked" hx-confirm="Are you sure you want to delete the selected files?">Delete Selected</button>
                <!-- Bulk download is complex to implement robustly and is omitted for simplicity -->
            </div>
        </form>

        <div class="upload-form">
            <h2>Upload Files</h2>
            <form hx-encoding="multipart/form-data" hx-post="/upload" hx-target="body">
                <label class="custom-file-upload">
                    <input type="file" name="files" multiple
                           class="file-input"
                           hx-trigger="change"
                           hx-encoding="multipart/form-data"
                           hx-post="/upload"
                           hx-target="body">
                    Upload files
                </label>
                <progress id="progress" value="0" max="100" style="display: none;"></progress>
            </form>
        </div>
    </div>

    <!-- Download notification element -->
    <div id="download-notification" class="download-notification"></div>

    <script>
      document.body.addEventListener('htmx:xhr:progress', function(evt) {
        var progress = document.getElementById('progress');
        progress.style.display = 'block';
        progress.value = evt.detail.loaded / evt.detail.total * 100;
      });
      document.body.addEventListener('htmx:afterRequest', function(evt) {
        var progress = document.getElementById('progress');
        if (progress) {
            setTimeout(function() {
                progress.style.display = 'none';
                progress.value = 0;
            }, 1000);
        }
      });

      // Function to show the download started notification
      function showDownloadStarted(filename) {
        var notification = document.getElementById('download-notification');
        notification.textContent = 'Downloading: ' + filename;
        notification.style.display = 'block';
        notification.style.opacity = '1';
        notification.style.animation = 'none';
        
        // Reset the animation
        setTimeout(function() {
          notification.style.animation = 'fadeOut 3s forwards';
        }, 100);
        
        // Hide the notification after animation completes
        setTimeout(function() {
          notification.style.display = 'none';
        }, 5000);
      }
    </script>
</body>
</html>
`
