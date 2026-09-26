# npcai

`npcai` é um servidor gRPC stateless para decisão de NPCs por Utility AI. Ele é iniciado pelo cliente do jogo como subprocesso: não há autenticação, multi-tenancy, container ou banco de dados. O estado de cada NPC vem no payload; perfis de tuning são carregados uma vez e consultados pelo ID.

## Rodar

Requer Go e [Buf](https://buf.build/docs/installation/).

```sh
make generate
go test ./...
go run ./cmd/npcai --config-dir configs
```

Por padrão o processo escuta em um socket Unix no macOS/Linux. No Windows, `uds` significa named pipe. Para desenvolvimento TCP use:

```sh
go run ./cmd/npcai --transport=tcp --port=0 --config-dir configs
```

A primeira e única linha escrita em stdout durante a inicialização é o handshake JSON; logs estruturados vão para stderr:

```json
{"transport":"uds","addr":"/tmp/npcai-123.sock","pid":123,"version":"dev"}
```

O cliente deve aguardar essa linha antes de conectar. O serviço registra o health check padrão gRPC (`grpc.health.v1.Health/Check`) e responde `SERVING` depois do handshake. `SIGINT`/`SIGTERM` marca o servidor como não servindo, faz `GracefulStop` e remove o socket Unix.

## Contrato e desempenho

O schema está em `proto/npcai/v1/decision.proto`; `gen/go/` é exclusivamente gerado por `buf generate`. `BatchDecide` é a rota de produção. `Decide` aceita exatamente um agente para depuração/tuning.

O batch usa SoA: `agent_ids`, posições e valores de consideração são arrays paralelos packed. Os valores de consideração usam a ordem declarada no perfil. Provedores e anúncios também são arrays paralelos; os offsets delimitam tags e deltas. Isso evita uma árvore `repeated Agent`, cujo custo de serialização se torna dominante em lotes grandes.

O servidor mantém os quatro perfis em memória, usa um worker pool limitado a `GOMAXPROCS` por batch e reutiliza estruturas temporárias com `sync.Pool`. Os benchmarks medem tanto `ScoreAction` quanto encode/decode protobuf:

```sh
make bench
```

## Simulação e escala

O motor preserva o ciclo `Consideration → response curve → utility → stochastic selection`. `FULL` aplica updaters e deltas graduais em ticks finos; `SIMPLIFIED` usa eventos agregados e reconstrói estado plausível ao voltar a `FULL`. Assim, NPCs distantes não consomem uma decisão fina a cada tick.

Perfis entregues:

- `social-life`: maior temperatura e emergência narrativa;
- `tactical-stealth`: temperatura e margem de preempção baixas;
- `survival-crafting`: custos e limiares críticos mais relevantes;
- `open-world-rpg`: LOD agressivo e compromissos narrativos fortes.

## Desenvolvimento

```sh
make generate  # Buf + plugins Go locais
make lint      # buf lint + golangci-lint
make test
make bench
make build
make build-all # linux/amd64, darwin/arm64, windows/amd64
```

`buf.gen.yaml` contém o ponto explícito para adicionar geração C# ou C++ no futuro; hoje só gera Go. A CI executa geração verificável, `buf lint`, `buf breaking`, testes com `-race`, `golangci-lint` e build.

## Próximo passo de escala: memória compartilhada

Memória compartilhada/arena não faz parte desta entrega. Antes de adotá-la, mediríamos com os benchmarks: bytes e alocações de marshal/unmarshal por agente, percentil de latência de `BatchDecide`, CPU de serialização versus scoring, tamanho médio dos batches e custo de cópia entre processo cliente/servidor. Só se serialização/cópia continuar dominante após payload SoA e batching, um ring buffer com versionamento, backpressure, ownership e recuperação de subprocesso justificaria a complexidade adicional.
