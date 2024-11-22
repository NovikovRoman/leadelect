# LEADer ELECTion

Нода может иметь статус `leader`, `candidate` или `follower`.
Нода `leader` выбирается **только** большинством нод.
Все ноды при старте имеют статус `follower`.

…

## Пример для запуска на локальной машине

- [cfg.yaml](#cfgyaml)
- [main.go](#maingo)
- [Сборка и запуск](#сборка-и-запуск)

### cfg.yaml

```yaml
certs:
  ca: certs/ca-cert.pem
  server:
    cert: certs/server-cert.pem
    key: certs/server-key.pem
nodes:
  1:
    addr: 127.0.0.1
    port: 50001
  2:
    addr: 127.0.0.1
    port: 50002
  3:
    addr: 127.0.0.1
    port: 50003
```

### main.go

```go
package main

import (
    "context"
    "fmt"
    "log"
    "log/slog"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/NovikovRoman/leadelect/node"
    "gopkg.in/yaml.v3"
)

type config struct {
    Certs struct {
        Ca     string `yaml:"ca"`
        Server struct {
            Cert string `yaml:"cert"`
            Key  string `yaml:"key"`
        } `yaml:"server"`
    }
    Nodes map[string]struct {
        Addr string `yaml:"addr"`
        Port string `yaml:"port"`
    } `yaml:"nodes"`
}

func main() {
    b, err := os.ReadFile("cfg.yaml")
    if err != nil {
        slog.Error(err.Error())
        os.Exit(1)
    }

    var cfg config
    if err = yaml.Unmarshal(b, &cfg); err != nil {
        slog.Error(err.Error())
        os.Exit(1)
    }

    p, ok := cfg.Nodes[os.Args[1]]
    if !ok {
        slog.Error("node not found")
        os.Exit(1)
    }

    opts := []node.NodeOpt{
        node.ClientTimeout(time.Second * 10),
        node.HeartBeatTimeout(time.Second * 3),
        node.CheckElectionTimeout(time.Second * 10),
        node.WithLogger(nil), // default slog
    }

    // with custom slog:
    // var buf bytes.Buffer
    // w := bufio.NewWriter(&buf)
    // customSlog := slog.NewTextHandler(w, nil)
    // opts = append(opts, node.WithLogger(node.NewLogger(customSlog)))

    port, _ := strconv.ParseInt(p.Port, 10, 64)
    n := node.New(os.Args[1], p.Addr, int(port), opts...)

    if cfg.Certs.Ca != "" {
        if err = n.ClientTLS(cfg.Certs.Ca, p.Addr); err != nil {
            slog.Error(err.Error())
            os.Exit(1)
        }
    }
    if cfg.Certs.Server.Cert != "" {
        if err = n.ServerTLS(cfg.Certs.Server.Cert, cfg.Certs.Server.Key); err != nil {
            slog.Error(err.Error())
            os.Exit(1)
        }
    }

    for id, v := range cfg.Nodes {
        if id == os.Args[1] {
            continue
        }
        port, _ := strconv.ParseInt(v.Port, 10, 64)
        n.AddNode(node.New(id, v.Addr, int(port)))
    }

    ctx, cancel := context.WithCancel(context.Background())
    go n.Run(ctx)

    go func() {
        for {
            time.Sleep(time.Second * 10)
            fmt.Println("Node status", n.Status())
        }
    }()

    shutdown := make(chan bool)
    defer close(shutdown)
    interrupt := make(chan os.Signal, 1)
    defer close(interrupt)
    signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-interrupt
        cancel()
        log.Println("Shutting down...")
        time.Sleep(time.Second * 3)
        shutdown <- true
    }()

    <-shutdown
    log.Println("Completed")
}
```

### Сборка и запуск

Сборка:

```shell
go build -o app
```

Сгенерировать сертификаты:

```shell
./gen_cert.sh
```

Запуск в разных консолях:

```shell
./app 1
./app 2
./app 3
```

### Генерация proto

```shell
protoc --go-grpc_out=. --go_out=. */*.proto
```

### Тестирование

```shell
go test ./node
```
