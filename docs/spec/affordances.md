# Protocolo de anúncio de affordances

## 5. Protocolo de anúncio de affordances

### 5.1 Ciclo de vida do anúncio

1. **Registro:** ao ser instanciado/ativado no mundo, o provedor publica sua lista de `AdvertisedAction` num índice espacial consultável ([§8.2](execution.md#82-fila-de-ações-actionqueue)). `[PREMISSA]` (mecanismo de descoberta)
2. **Descoberta:** quando o agente entra em estado de decisão ([§8.3](execution.md#83-interrupção-preempção)), coleta todos os anúncios cujo `advertisement_radius` cobre sua posição e cujas `preconditions` ele satisfaz.
3. **Avaliação:** cada anúncio elegível recebe uma pontuação ([§6](selection.md#6-algoritmo-de-seleção-de-ação)).
4. **Reserva:** ao escolher, o agente ocupa uma vaga do provedor (`capacity`) **e** uma vaga da ação (`capacity` da `AdvertisedAction`, [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)); anúncios sem vaga livre em qualquer um dos dois níveis deixam de ser elegíveis para outros agentes. Disputas dentro de um mesmo lote de decisão são arbitradas pelo serviço ([§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote)).
5. **Execução e entrega:** durante a execução, os `deltas` são aplicados **gradualmente** (taxa = delta / `estimated_duration` por unidade de tempo) até completar, ser interrompido ou saturar a consideração. Custos (`cost`) são pagos no início ou ao longo da execução. `[PREMISSA]`
6. **Revogação:** provedor removido/destruído/descarregado revoga seus anúncios; agentes a caminho reavaliam. `[PREMISSA]`

**Divisão de responsabilidades.** A descoberta grossa — índice espacial, ativação por região, streaming de provedores — pertence ao mundo/cliente e é `[PREMISSA]` de mecanismo. A regra do protocolo começa na **elegibilidade** ([§5.5](affordances.md#55-elegibilidade-o-que-é-verificado-antes-e-depois-de-pontuar)): mesmo que o índice entregue um provedor, o alcance é verificado de novo na elegibilidade, de forma **inclusiva** (distância igual ao raio cobre). O motor nunca confia na pré-seleção.

### 5.2 Anúncios dinâmicos e condicionais

Provedores **reavaliam suas `preconditions`** e podem alterar, suspender ou criar anúncios conforme o estado do mundo e do candidato:

- **Condicional ao agente:** a porta trancada só anuncia `break_in` para quem tem skill `lockpicking`; o ferreiro só anuncia `forge` para quem tem a receita; o altar só anuncia `pray` para a facção certa.
- **Condicional ao provedor:** arma sem munição não anuncia `shoot`; cama ocupada não anuncia; zona de cobertura destruída revoga `take_cover`.
- **Condicional ao contexto global:** durante um alarme, guardas passam a anunciar `investigate`; durante um festival, praças anunciam `celebrate`.
- **Agentes como provedores:** um NPC ocioso anuncia `converse` (delta em `BOND`/`SOCIAL`); um NPC hostil anuncia `duel`; um mercador anuncia `trade`.

**Limite do contrato de lote atual.** Precondições, custo e estado do provedor fazem parte do modelo, mas o contrato de decisão em lote da implementação de referência ainda **não os transporta**: o lote carrega, por ação, apenas identidade, `deltas`, duração, tags, domínio, prioridade, raio, capacidade e ocupação; e, por agente, apenas identidade, posição e valores de consideração. Consequência prática: no caminho servido, a elegibilidade efetiva se reduz a alcance e vagas nos dois níveis, e a filtragem por capability/recurso/estado só pode acontecer **no cliente, antes do anúncio** — como o lote é compartilhado entre agentes, filtrar por agente exige lotes separados por conjunto de capacidades, o que é grosseiro. O modelo especifica esses campos para que a extensão do contrato seja só de transporte, não de semântica.

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

**Vagas de ação são por instância de provedor.** O limite de `saw` na bancada X não se soma nem se compartilha com o `saw` da bancada Y: cada provedor carrega seus próprios contadores, e duas ações com o mesmo `action_id` em provedores diferentes disputam apenas as vagas do respectivo provedor.

**Ocupação acima da capacidade** (`occupancy` > `capacity`) não é erro: a ação é tratada como saturada, com 0 vagas livres — o excedente simplesmente não cabe no retrato.

**No contrato de lote**, os arrays de capacidade e de ocupação por ação são **opcionais**: um array ausente lê-se como todos zeros (sem limite próprio, nenhum ocupante) — que é exatamente o que todo cliente anterior à introdução do campo envia. Já a capacidade do **provedor** é obrigatória no lote, e 0 ali bloqueia, coerente com a assimetria acima.

### 5.5 Elegibilidade: o que é verificado antes e depois de pontuar

Todas as verificações abaixo são **gates binários anteriores à pontuação**: falhar em qualquer um não diminui a nota do anúncio — ele deixa de existir para o agente naquele tick. Na ordem em que são aplicados:

1. **Vaga no provedor:** `capacity − |occupants| ≤ 0` descarta o provedor inteiro, com todas as suas ações.
2. **Vaga na ação:** `capacity > 0` e `occupancy ≥ capacity` descartam a ação ([§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)). Capacidade 0 nunca descarta.
3. **Alcance:** distância agente–provedor maior que `advertisement_radius` descarta; distância **igual** ao raio mantém (o teste é inclusivo).
4. **Precondições:** conjunção de predicados ([§3.5](data-model.md#35-advertisedaction)) — capability exigida presente no agente; recurso do agente ≥ `minimum`; `required_state` igual, por string exata, ao `state` do provedor.
5. **Custo pagável:** para cada recurso de `cost`, o estoque do agente cobre a quantidade. Recurso ausente do estoque lê-se como 0.
6. **Compromissos narrativos:** nenhum `NarrativeCommitment` ativo do agente lista como contraditória nenhuma tag da ação ([§8.2](execution.md#82-fila-de-ações-actionqueue)).

Só quem passa pelos seis gates é pontuado ([§6.2](selection.md#62-função-de-utilidade)). Depois da pontuação vêm, nesta ordem: corte nos `SELECTION_TOP_K` melhores, sorteio softmax ([§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)) e, em decisões em lote, a arbitragem de disputas ([§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote)) — que pode devolver o agente a um candidato pior da própria lista, mas nunca a um que tenha falhado num gate.

Note a assimetria de papel entre custo e capacidade: `cost` aparece **duas vezes** na vida de um anúncio — como gate (pagável ou não) e depois como penalidade na utilidade, que cresce com a escassez do recurso ([§6.2](selection.md#62-função-de-utilidade)). Capacidade, ao contrário, é só gate: vaga livre não bonifica a pontuação, e vaga quase cheia não a penaliza.

---
