# Desenvolvimento

## Rodar

Requer Go e [Buf](https://buf.build/docs/installation/).

```sh
make generate
go test ./...
go run ./cmd/geppetto --config-dir configs
```

Por padrão o processo escuta em um socket Unix no macOS/Linux (named pipe no Windows). Para desenvolvimento TCP:

```sh
go run ./cmd/geppetto --transport=tcp --port=0 --config-dir configs
```

O contrato de inicialização (handshake em stdout, health check, shutdown) está em [protocol.md](protocol.md).

## Tarefas do Makefile

```sh
make generate  # Buf + plugins Go locais
make lint      # buf lint + golangci-lint
make test
make bench
make build
make build-all # linux/amd64, darwin/arm64, windows/amd64
```

## Hot reload com air

Para desenvolvimento com rebuild automático, usamos o [air](https://github.com/air-verse/air) (repositório atual; o antigo `cosmtrek/air` foi migrado). É ferramenta de dev apenas: não é dependência de build nem de CI.

```sh
go install github.com/air-verse/air@latest
make dev
```

O `.air.toml` observa arquivos `.go` e `configs/*.json` (perfis são carregados uma única vez no startup, então editar um perfil reinicia o servidor para valer). Ficam fora do watch: `bin/`, `dist/`, `tmp/`, `gen/` (gerado pelo buf, só muda via `make generate`) e `*_test.go`.

**Endereço estável e limitação de reconexão.** Em produção o endereço muda a cada processo (socket por PID ou porta TCP efêmera) e o cliente o descobre pela linha de handshake no stdout. No modo dev o air sobe o servidor com `--socket tmp/geppetto-dev.sock`, um caminho fixo: a cada rebuild o novo processo publica o **mesmo** endereço no handshake. O cliente ainda **precisa reconectar** após cada reload — a conexão anterior morre com o processo antigo; o que o endereço estável elimina é a necessidade de redescobrir o endereço (reler o handshake) ou reiniciar o cliente. Se o cliente prefere TCP, a mesma ideia vale com `--transport=tcp --port=<porta fixa>`.

## Geração de código e CI

`buf.gen.yaml` contém o ponto explícito para adicionar geração C# ou C++ no futuro; hoje só gera Go. A CI executa geração verificável, `buf lint`, `buf breaking`, testes com `-race`, `golangci-lint` e build.
