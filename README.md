# LEADer ELECTion

> Библиотека выбора лидера в кластере.

Все ноды при старте имеют статус `follower`.
При старте все ноды проверяют статусы всех нод (инициализация нод),
если нет лидера запускается механизм голосования.
Нода `leader` выбирается **только** большинством нод.

Нода может иметь статус `leader`, `candidate` или `follower`.

Если у лидера осталось менее половины доступных нод, то он переходит в статус `follower`
и запускается механизм голосования.

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
        slog.Error(fmt.Sprintf("failed to read configuration file: %v", err))
        os.Exit(1)
    }

    var cfg config
    if err = yaml.Unmarshal(b, &cfg); err != nil {
        slog.Error(fmt.Sprintf("failed to parse configuration file: %v", err))
        os.Exit(1)
    }

    // Ensure node ID is provided as an argument
    if len(os.Args) < 2 {
        slog.Error("node ID argument missing")
        os.Exit(1)
    }

    currentNodeID := os.Args[1]
    cfgCurrNode, ok := cfg.Nodes[os.Args[1]]
    if !ok {
        slog.Error(fmt.Sprintf("node %s not found in configuration", currentNodeID))
        os.Exit(1)
    }

    opts := []node.NodeOpt{
        node.ClientTimeout(time.Second * 10),
        node.HeartbeatTimeout(time.Second * 3),
        node.CheckElectionTimeout(time.Second * 10),
        node.WithLogger(nil), // default slog
    }

    // with custom slog:
    // var buf bytes.Buffer
    // w := bufio.NewWriter(&buf)
    // customSlog := slog.NewTextHandler(w, nil)
    // opts = append(opts, node.WithLogger(node.NewLogger(customSlog)))

    port, _ := strconv.ParseInt(cfgCurrNode.Port, 10, 64)
    currNode := node.New(currentNodeID, cfgCurrNode.Addr, int(port), opts...)

    // Setup TLS if specified in configuration
    if cfg.Certs.Ca != "" {
        if err = currNode.ClientTLS(cfg.Certs.Ca, cfgCurrNode.Addr); err != nil {
            slog.Error(fmt.Sprintf("failed to set up client TLS %v", err))
            os.Exit(1)
        }
    }
    if cfg.Certs.Server.Cert != "" {
        if err = currNode.ServerTLS(cfg.Certs.Server.Cert, cfg.Certs.Server.Key); err != nil {
            slog.Error(fmt.Sprintf("failed to set up server TLS %v", err))
            os.Exit(1)
        }
    }

    for id, v := range cfg.Nodes {
        if id == currentNodeID {
            continue
        }
        p, err := strconv.Atoi(v.Port)
        if err != nil {
            slog.Warn(fmt.Sprintf("invalid port %d for node %s, skipping", p, id))
            continue
        }
        currNode.AddNode(node.New(id, v.Addr, p))
    }

    ctx, cancel := context.WithCancel(context.Background())
    // Start the node
    go currNode.Run(ctx)

    go func() {
        for {
         // …
         // your code
         // …

         // this code is an example
         time.Sleep(time.Second * 10)
         fmt.Println("Node status", currNode.Status())
     }
    }()

    // Handle graceful shutdown
    shutdown := make(chan bool)
    defer close(shutdown)
    interrupt := make(chan os.Signal, 1)
    defer close(interrupt)
    signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-interrupt
        cancel()
        log.Println("Shutting down...")
        // time to complete the process
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
