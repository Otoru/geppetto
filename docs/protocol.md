# Protocolo e contrato de wire

Este documento descreve o contrato entre o processo do jogo (cliente) e o processo `geppetto` (servidor): transporte, handshake de inicialização, schema gRPC e o formato do payload de decisão em lote.

## Transporte e handshake

Por padrão o processo escuta em um socket Unix no macOS/Linux. No Windows, `uds` significa named pipe. Para desenvolvimento TCP use:

```sh
go run ./cmd/geppetto --transport=tcp --port=0 --config-dir configs
```

A primeira e única linha escrita em stdout durante a inicialização é o handshake JSON; logs estruturados vão para stderr:

```json
{"transport":"uds","addr":"/tmp/geppetto-123.sock","pid":123,"version":"dev"}
```

O cliente deve aguardar essa linha antes de conectar. O serviço registra o health check padrão gRPC (`grpc.health.v1.Health/Check`) e responde `SERVING` depois do handshake. `SIGINT`/`SIGTERM` marca o servidor como não servindo, faz `GracefulStop` e remove o socket Unix.

## Schema gRPC

O schema está em `proto/geppetto/v1/decision.proto`; `gen/go/` é exclusivamente gerado por `buf generate`. `BatchDecide` é a rota de produção. `Decide` aceita exatamente um agente para depuração/tuning.

### Renomeação do serviço gRPC

Esta renomeação muda deliberadamente o serviço na wire de `npcai.v1.DecisionService` para `geppetto.v1.DecisionService`, antes de existir qualquer consumidor. Por isso, `buf breaking --against ".git#branch=main"` acusa esta alteração neste commit; a guarda permanece ativa para mudanças futuras.

### Capacidade por ação (campos 31 e 32)

O batch ganhou `action_capacities` e `action_occupancies`, arrays paralelos alinhados por índice de ação. A adição é compatível na wire — `buf breaking` não acusa, e não deve: campos novos não quebram consumidores existentes.

**O zero tem semântica oposta à do provedor, de propósito.** Em `provider_capacities`, 0 bloqueia o provedor. Em `action_capacities`, **0 significa SEM LIMITE**: proto3 codifica campo numérico ausente como 0 e todo cliente anterior a este campo envia 0 — se 0 bloqueasse, toda ação existente viraria inutilizável no instante do deploy. Um agente só entra numa ação com vaga nos DOIS níveis (provedor E ação); o mais restritivo manda. Array vazio inteiro também vale como "tudo zero" (cliente antigo), e o servidor valida que, quando presentes, os arrays têm o comprimento de `action_ids`.

`action_occupancies` informa quantos agentes já executam cada ação no início do tick, para o limite da ação valer contra o retrato inicial e não só dentro do lote.

## Formato do batch (SoA)

O batch usa SoA: `agent_ids`, posições e valores de consideração são arrays paralelos packed. Os valores de consideração usam a ordem declarada no perfil. Provedores e anúncios também são arrays paralelos; os offsets delimitam tags e deltas.

Um batch tem exatamente um `profile_id`: como os `consideration_values` seguem a ordem declarada pelo perfil, perfis não podem ser misturados no mesmo batch — um valor em um dado offset não teria significado inequívoco.

A justificativa de desempenho deste layout (e o que acontece em lotes de 50k+ agentes se ele for simplificado) está em [scaling.md](scaling.md).
