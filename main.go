package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- Domain Models ---

type Post struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// --- In-Memory Database (Thread-Safe) ---

type Database struct {
	mu    sync.RWMutex
	posts []Post
}

var db = Database{
	posts: []Post{
		{
			ID:        "1",
			Title:     "Welcome to the Go Blog",
			Content:   "This is a simple blog scaffolded with Go and Alpine.js. It's fast, single-file, and thread-safe!",
			CreatedAt: time.Now(),
		},
	},
}

// --- Middleware ---

// LoggerMiddleware logs the method, path, and duration of each request
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Wrap ResponseWriter to capture status code
		ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(ww, r)

		log.Printf("[%s] %s %s took %v", r.Method, ww.statusCode, r.URL.Path, time.Since(start))
	})
}

// SecurityMiddleware adds basic security headers
func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// responseWriterWrapper captures the status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int // Default 200
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterWrapper) String() string {
	return fmt.Sprintf("%d", rw.statusCode)
}

// --- Handlers ---

func handlePosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		db.mu.RLock()
		defer db.mu.RUnlock()
		json.NewEncoder(w).Encode(db.posts)

	case http.MethodPost:
		var p Post
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Simple ID generation and validation
		p.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		p.CreatedAt = time.Now()
		
		if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Content) == "" {
			http.Error(w, "Title and Content are required", http.StatusBadRequest)
			return
		}

		db.mu.Lock()
		// Prepend to show newest first
		db.posts = append([]Post{p}, db.posts...)
		db.mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlFrontend))
}

// --- Main ---

func main() {
	mux := http.NewServeMux()

	// Register Routes
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/api/posts", handlePosts)

	// Apply Middleware Chain
	handler := LoggerMiddleware(SecurityMiddleware(mux))

	port := ":8080"
	fmt.Printf("Starting server on http://localhost%s\n", port)
	if err := http.ListenAndServe(port, handler); err != nil {
		log.Fatal(err)
	}
}

// --- Embedded Frontend (Alpine.js + Tailwind) ---

const htmlFrontend = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go + Alpine Blog</title>
    <!-- Tailwind CSS -->
    <script src="https://cdn.tailwindcss.com"></script>
    <!-- Alpine.js -->
    <script src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
</head>
<body class="bg-gray-50 text-gray-800 antialiased" x-data="blogApp()">

    <!-- Navigation -->
    <nav class="bg-blue-600 text-white p-4 shadow-md">
        <div class="container mx-auto flex justify-between items-center">
            <h1 class="text-xl font-bold">GoBlog</h1>
            <span class="text-sm opacity-80">Running on Go stdlib</span>
        </div>
    </nav>

    <div class="container mx-auto p-4 max-w-3xl">
        
        <!-- Create Post Form -->
        <div class="bg-white p-6 rounded-lg shadow-sm mb-8 border border-gray-100">
            <h2 class="text-lg font-semibold mb-4 text-gray-700">Write a new post</h2>
            <form @submit.prevent="createPost">
                <div class="mb-4">
                    <label class="block text-sm font-medium mb-1">Title</label>
                    <input type="text" x-model="newPost.title" class="w-full p-2 border rounded focus:ring-2 focus:ring-blue-500 outline-none" placeholder="Enter title..." required>
                </div>
                <div class="mb-4">
                    <label class="block text-sm font-medium mb-1">Content</label>
                    <textarea x-model="newPost.content" class="w-full p-2 border rounded h-24 focus:ring-2 focus:ring-blue-500 outline-none" placeholder="What's on your mind?" required></textarea>
                </div>
                <div class="flex justify-end">
                    <button type="submit" class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded transition" :disabled="isLoading">
                        <span x-show="!isLoading">Publish Post</span>
                        <span x-show="isLoading">Publishing...</span>
                    </button>
                </div>
            </form>
        </div>

        <!-- Posts List -->
        <div class="space-y-6">
            <template x-for="post in posts" :key="post.id">
                <article class="bg-white p-6 rounded-lg shadow-sm border border-gray-100 hover:shadow-md transition">
                    <div class="flex justify-between items-start mb-2">
                        <h2 class="text-xl font-bold text-gray-900" x-text="post.title"></h2>
                        <span class="text-xs text-gray-500" x-text="formatDate(post.created_at)"></span>
                    </div>
                    <p class="text-gray-600 leading-relaxed whitespace-pre-line" x-text="post.content"></p>
                </article>
            </template>

            <!-- Loading State -->
            <div x-show="posts.length === 0 && !isLoading" class="text-center text-gray-500 py-10">
                No posts yet. Be the first to write one!
            </div>
        </div>

    </div>

    <script>
        function blogApp() {
            return {
                posts: [],
                newPost: { title: '', content: '' },
                isLoading: false,

                init() {
                    this.fetchPosts();
                },

                async fetchPosts() {
                    try {
                        const res = await fetch('/api/posts');
                        if (res.ok) {
                            this.posts = await res.json();
                        }
                    } catch (e) {
                        console.error("Failed to fetch posts", e);
                    }
                },

                async createPost() {
                    this.isLoading = true;
                    try {
                        const res = await fetch('/api/posts', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify(this.newPost)
                        });

                        if (res.ok) {
                            const createdPost = await res.json();
                            // Add new post to top of list
                            this.posts.unshift(createdPost); 
                            // Reset form
                            this.newPost = { title: '', content: '' };
                        }
                    } catch (e) {
                        console.error("Failed to create post", e);
                    } finally {
                        this.isLoading = false;
                    }
                },

                formatDate(dateStr) {
                    const date = new Date(dateStr);
                    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
                }
            }
        }
    </script>
</body>
</html>
`