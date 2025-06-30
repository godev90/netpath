# NetPath

**NetPath** is a lightweight HTTP routing and middleware framework for Go, designed with simplicity and modularity in mind. It provides an expressive way to define routes, middleware chains, and structured request context handling—ideal for building web services or APIs.

---

## ✨ Features

- Custom App, Router, and Context architecture
- Clean middleware chaining (inspired by Negroni/Express.js style)
- Route parameter matching
- Built-in request validation support (via godev90/validator)
- Modular structure with support for PostgreSQL and MySQL drivers via GORM
- Lightweight and fast, with minimal external dependencies

---

## 📦 Installation

go get github.com/godev90/netpath

---

## 🚀 Quick Start
```go
package main

import (
    "net/http"
    "github.com/godev90/netpath"
)

func main() {
    app := netpath.New()
    
    app.Route().Handle("GET", "/hello", func(ctx *netpath.Context) error {
        return ctx.JSON(http.StatusOK, map[string]string{"message": "Hello, world!"})
    })

    http.ListenAndServe(":8080", app)
}
```

--- 

## 🔧 Middleware Example
func loggingMiddleware(next netpath.HandlerFunc) netpath.HandlerFunc {
    return func(ctx *netpath.Context) error {
        fmt.Println("Before request")
        err := next(ctx)
        fmt.Println("After request")
        return err
    }
}

app.Use(loggingMiddleware)

---

## 🛠 Dependencies
- GORM for DB support
- Validator for request validation
- Go modules (go 1.23.4)

---

## 🤝 Contributing

Pull requests are welcome! Please open an issue first to discuss your idea.

---