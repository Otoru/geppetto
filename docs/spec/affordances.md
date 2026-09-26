# Protocolo de anúncio de affordances

## 5. Protocolo de anúncio de affordances

### 5.1 Ciclo de vida do anúncio

1. **Registro:** ao ser instanciado/ativado no mundo, o provedor publica sua lista de `AdvertisedAction` num índice espacial consultável ([§8.2](execution.md#82-fila-de-ações-actionqueue)). `[PREMISSA]` (mecanismo de descoberta)
2. **Descoberta:** quando o agente entra em estado de decisão ([§8.3](execution.md#83-interrupção-preempção)), coleta todos os anúncios cujo `advertisement_radius` cobre sua posição e cujas `preconditions` ele satisfaz.
3. **Avaliação:** cada anúncio elegível recebe uma pontuação ([§6](selection.md#6-algoritmo-de-seleção-de-ação)).
4. **Reserva:** ao escolher, o agente ocupa uma vaga do provedor (`capacity`) **e** uma vaga da ação (`capacity` da `AdvertisedAction`, [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)); anúncios sem vaga livre em qualquer um dos dois níveis deixam de ser elegíveis para outros agentes. Disputas dentro de um mesmo lote de decisão são arbitradas pelo serviço ([§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote)).
5. **Execução e entrega:** durante a execução, os `deltas` são aplicados **gradualmente** (taxa = delta / `estimated_duration` por unidade de tempo) até completar, ser interrompido ou saturar a consideração. Custos (`cost`) são pagos no início ou ao longo da execução. `[PREMISSA]`
6. **Revogação:** provedor removido/destruído/descarregado revoga seus anúncios; agentes a caminho reavaliam. `[PREMISSA]`

### 5.2 Anúncios dinâmicos e condicionais

Provedores **reavaliam suas `preconditions`** e podem alterar, suspender ou criar anúncios conforme o estado do mundo e do candidato:

- **Condicional ao agente:** a porta trancada só anuncia `break_in` para quem tem skill `lockpicking`; o ferreiro só anuncia `forge` para quem tem a receita; o altar só anuncia `pray` para a facção certa.
- **Condicional ao provedor:** arma sem munição não anuncia `shoot`; cama ocupada não anuncia; zona de cobertura destruída revoga `take_cover`.
- **Condicional ao contexto global:** durante um alarme, guardas passam a anunciar `investigate`; durante um festival, praças anunciam `celebrate`.
- **Agentes como provedores:** um NPC ocioso anuncia `converse` (delta em `BOND`/`SOCIAL`); um NPC hostil anuncia `duel`; um mercador anuncia `trade`.

### 5.3 Regras do protocolo

- O agente **nunca** consulta tabela global de entidades; só vê o que está anunciado ao alcance. É isso que permite adicionar conteúdo sem tocar no agente.
- Anúncios são **declarativos**: o provedor promete deltas; cabe ao agente ponderar se a promessa vale agora, dadas suas considerações, traços e contextos.
- Deltas podem ser **negativos ou mistos**: `fight_in_arena` promete +`FUN`, −`ENERGY`, +`INJURY`. A utilidade soma todos os efeitos ponderados.

### 5.4 Capacidade em dois níveis e a semântica do zero

Capacidade existe em **dois níveis independentes**, e um agente só entra numa ação com vaga livre nos dois:

- **Provedor** (`AffordanceProvider.capacity`): quantos agentes o provedor aceita no total, somando todas as ações que ele anuncia. **0 bloqueia** o provedor.
- **Ação** (`AdvertisedAction.capacity`): quantos agentes podem executar aquela ação específica. **0 = SEM LIMITE** — a ação não impõe restrição própria e vale só o limite do provedor.

O mais restritivo manda: uma ação com `capacity` 3 num provedor com `capacity` 1 admite 1 agente; uma bancada com `capacity` 4 cuja ação `saw` tem `capacity` 1 admite 4 agentes no total, mas só 1 serrando.

**A assimetria do zero é deliberada e merece destaque, não nota de rodapé.** No provedor, 0 bloqueia; na ação, 0 libera. O motivo é mecânico: o contrato de wire é proto3, que codifica campo numérico ausente como 0. Quando a capacidade por ação foi introduzida, todo cliente já existente passou a enviar 0 sem saber. Se 0 bloqueasse a ação, **toda ação de todo jogo existente viraria inutilizável no instante do deploy**. A alternativa "consistente" (0 bloqueia nos dois níveis) é a que destrói retrocompatibilidade; a assimetria é o preço de adicionar o campo sem quebrar ninguém. Quem estender este modelo deve manter: campo de capacidade novo entra com 0 = sem limite, ou não entra.

**Ocupação por ação.** O cliente informa, por ação, quantos agentes já a executam no início do tick (`occupancy`). Isso faz o limite da ação valer também contra o retrato inicial do mundo, e não apenas contra as decisões do lote em curso. A ocupação por ação é uma contagem; a identidade dos ocupantes não é relevante para a arbitragem.

---
