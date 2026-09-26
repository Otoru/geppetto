# Escala e desempenho

## Por que o payload é SoA

O batch usa SoA (structure of arrays) em vez de uma árvore `repeated Agent`: `agent_ids`, posições e valores de consideração são arrays paralelos packed, e provedores e anúncios seguem o mesmo padrão com offsets delimitando tags e deltas (o formato exato está em [protocol.md](protocol.md)). Isso evita uma árvore `repeated Agent`, cujo custo de serialização se torna dominante em lotes grandes.

O comentário no schema (`proto/geppetto/v1/decision.proto`) registra a consequência de simplificar esse layout:

> This is a structure-of-arrays (SoA) payload: parallel packed arrays avoid a `repeated Agent` envelope dominating serialization and allocation costs. At 50k+ agents per tick, replacing it with one message per agent would make the wire and GC overhead part of the decision budget again.

Ou seja: com 50 mil ou mais agentes por tick, voltar a uma mensagem por agente recoloca o overhead de wire e de GC dentro do orçamento da decisão. O layout SoA existe para que serialização e alocação não virem o gargalo.

## O que o servidor faz para sustentar isso

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

## Próximo passo de escala: memória compartilhada

Memória compartilhada/arena não faz parte desta entrega. Antes de adotá-la, mediríamos com os benchmarks: bytes e alocações de marshal/unmarshal por agente, percentil de latência de `BatchDecide`, CPU de serialização versus scoring, tamanho médio dos batches e custo de cópia entre processo cliente/servidor. Só se serialização/cópia continuar dominante após payload SoA e batching, um ring buffer com versionamento, backpressure, ownership e recuperação de subprocesso justificaria a complexidade adicional.
