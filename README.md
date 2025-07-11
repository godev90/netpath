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
```bash
go get github.com/godev90/netpath
```

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
```go
func loggingMiddleware(next netpath.HandlerFunc) netpath.HandlerFunc {
    return func(ctx *netpath.Context) error {
        fmt.Println("Before request")
        err := next(ctx)
        fmt.Println("After request")
        return err
    }
}

app.Use(loggingMiddleware)
```

## 🗂 Session Support
**NetPath** supports storing session information in the request context by implementing the Session interface.
You can define your own session struct and attach it to the context using middleware.

```go
// this interface
type SessionType uint

type Session interface {
    Identifier() string
    Type() SessionType
}
```

### Example
```go
const (
    SessionTypeUser SessionType = iota
    SessionTypeAdmin
)

type UserSession struct {
    UserID string
}

func (s *UserSession) Identifier() string {
    return s.UserID
}

func (s *UserSession) Type() SessionType {
    return SessionTypeUser
}
```

### Middleware Example
Attach a session to the context using middleware:
```go
func sessionMiddleware(next netpath.HandlerFunc) netpath.HandlerFunc {
    return func(ctx *netpath.Context) error {
        session := &MySession{
            UserID: "123",
            Role:   SessionTypeAdmin,
        }
        ctx.SetSession(session)
        return next(ctx)
    }
}

app.Use(sessionMiddleware)
```

### Accessing Session in Handlers
```go
session := ctx.Session().(*MySession)
fmt.Println("User ID:", session.Identifier())
fmt.Println("Session Type:", session.Type())
```

---

## 🔐 Registering a Session Type

Before using a session in your application, you must register its type using the RegisterSessionType function. This ensures type safety and prevents conflicting session types for the same SessionType identifier.

```go
func RegisterSessionType(session Session)
```

Purpose
- Ensures only one session struct is used for each SessionType
- Panics if:
  - The provided session is nil
  - A different struct has already been registered for the same SessionType


### Example 
```go
func init() {
    RegisterSessionType(&MySession{})
}
```

---

## 🛠 Dependencies
- GORM for DB support
- Validator for request validation
- Go modules (go 1.23.4)

---

## 🤝 Contributing

Pull requests are welcome! Please open an issue first to discuss your idea.

---