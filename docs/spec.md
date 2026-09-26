# Especificação Técnica — Arquitetura de IA Utilitária para NPCs (Utility AI com Affordances)

**Escopo:** arquitetura genérica de decisão autônoma para NPCs de qualquer gênero de jogo (RPG, survival, stealth, tático, imersivo, estratégia, simulação social, mundo aberto).
**Núcleo:** `considerations → response curves → utility → selection`. O ambiente anuncia ações; o agente pontua e escolhe — deliberadamente sem escolher sempre a melhor.
**Marcações:** `[PREMISSA]` indica decisão de engenharia arbitrária (valores default, forma de função, mecanismo de seleção). A proveniência desta spec está no Apêndice A.
**Implementação de referência:** o projeto [geppetto](https://github.com/vitorhugo/geppetto) implementa esta especificação; este documento é versionado junto do código (ver Apêndice A).

---

## 1. Visão geral e objetivos

### 1.1 O que é o sistema

Um sistema de **agentes autônomos baseados em utilidade**. Cada agente (NPC) mantém um conjunto de **considerações** (`Consideration`) — sinais escalares normalizados que descrevem seu estado interno e sua leitura do mundo (fome, ameaça, munição, cobertura, vínculos, objetivos de facção, medo, fadiga…). As **entidades do mundo** atuam como **provedores de affordance** (`AffordanceProvider`): anunciam as ações que oferecem e os deltas que prometem sobre as considerações. Quando o agente decide, ele **pontua as ações anunciadas ao alcance** e escolhe uma das mais bem pontuadas — **não necessariamente a melhor**.

A inversão arquitetural central: o agente **não conhece** as entidades do mundo. Não existe código no agente do tipo "se com fome, procure geladeira" nem "se sob fogo, procure cobertura". O conhecimento sobre o que cada coisa oferece vive **nos provedores**. Novas entidades, itens, locais e eventos entram no sistema sem alterar o "cérebro" dos agentes.

### 1.2 Objetivos

1. **Autonomia plausível:** o agente decide sozinho em qualquer ambiente, com qualquer combinação de entidades, atores e situações.
2. **Extensibilidade:** adicionar um provedor de affordance novo (arma, abrigo, altar, ponto de patrulha, NPC conversável) não exige mudança no agente — basta o provedor declarar seus anúncios.
3. **Comportamento legível:** traços e preferências enviesam as escolhas de forma que um observador consiga *contar uma história* sobre por que o NPC fez aquilo.
4. **Emergência:** o sistema deve gerar situações inesperadas e memoráveis, não apenas comportamento correto.
5. **Escala:** suportar dezenas/centenas de agentes, simulando de forma barata os que estão longe do jogador (níveis de detalhe de simulação).

### 1.3 Não-objetivos (por que o agente NÃO deve ser ótimo)

- **Não maximizar eficiência.** Um NPC que executa sempre a política ótima (posicionamento perfeito, economia perfeita de recursos, rotina perfeita) elimina tensão, exploração e jogo.
- **Não ser previsível.** Seleção por argmax puro transforma NPCs em autômatos exploráveis: o jogador aprende a função e a manipula. Erro deliberado sustenta a ilusão de intenção.
- **Não ser correto demais.** Regras de comportamento perfeitamente seguidas produzem resultados previsíveis e sem graça; desvios ocasionais geram situações memoráveis.
- **Não simular tudo.** Uma boa simulação não precisa simular tudo; simular menos, nos lugares certos, melhora o jogo e o orçamento de CPU.
- **Não contradizer a narrativa do jogador.** A autonomia cuida da vida do NPC, mas não deve desfazer atos do jogador nem histórias em andamento (regra de improvisação: "aceite a ideia do jogador e continue a partir dela").

---

## 2. Glossário

| Termo | Definição |
|---|---|
| **Agente / NPC** (`Agent`) | Entidade autônoma com considerações, personalidade e fila de ações. |
| **Consideração** (`Consideration`) | Sinal escalar normalizado (tipicamente [0, 1] ou [-100, +100]) que alimenta a função de utilidade. Generaliza "necessidades": inclui estado fisiológico, ameaça, recursos, posicionamento, relações, objetivos, medo, estado de missão. |
| **Atualizador de consideração** (`ConsiderationUpdater`) | Regra que evolui uma consideração ao longo do tempo ou em resposta a eventos. O **decaimento temporal** é um tipo de atualizador — não o modelo inteiro. |
| **Curva de resposta** (`response_curve`) | Função que converte o valor bruto de uma consideração em **pressão** (peso) para a utilidade. |
| **Traço** (`Trait`) | Característica nomeada de personalidade (ex.: `cautious`, `sadistic`, `lazy`, `loyal`) que enviesa pontuações e pode criar ou modular considerações. |
| **Provedor de affordance** (`AffordanceProvider`) | Qualquer entidade do mundo que anuncia ações: objeto, local, zona, outro NPC, evento, item, ponto de patrulha, cobertura. |
| **Anúncio / ação anunciada** (`AdvertisedAction`) | Declaração, feita pelo provedor, de uma ação disponível + deltas prometidos sobre considerações + precondições. |
| **Utilidade / pontuação** (`utility`, calculada por `score_action()`) | Número calculado pelo agente para cada ação anunciada; base da seleção. |
| **Pressão / urgência** (`pressure`) | Peso de uma consideração após a curva de resposta; cresce conforme a situação se torna crítica. |
| **Seleção estocástica** | Escolha aleatória enviesada entre as ações mais bem pontuadas, em vez de argmax determinístico. |
| **Contexto situacional** (`Context`) | Modificador temporário de utilidade criado por local, papel social, estado de combate, missão ou facção. |
| **LOD de simulação** (`simulation_level`) | Agentes longe do jogador são simulados de forma simplificada (eventos agregados); ao voltar para perto, seu estado é reconstruído de forma plausível. |
| **Compromisso narrativo** (`NarrativeCommitment`) | Estado que a autonomia não deve destruir (ex.: aliança ou romance iniciado pelo jogador, missão em andamento). |

### 2.1 Convenção de nomenclatura

Todos os identificadores desta spec são em **inglês**, com o seguinte casing. Quem estender a spec (novas considerações, tags, parâmetros) deve seguir o mesmo padrão:

| Categoria | Convenção | Exemplos |
|---|---|---|
| Entidades / tipos / estruturas | PascalCase | `Agent`, `Consideration`, `AffordanceProvider`, `AdvertisedAction`, `Trait`, `ActionQueue` |
| Campos e parâmetros de tuning | snake_case | `decay_rate`, `preemption_margin`, `advertisement_radius`, `response_curve_exponent` |
| Valores de enum / constantes / nomes de consideração | SCREAMING_SNAKE_CASE | `HUNGER`, `ENERGY`, `THREAT`, `COVER`, `AMMO`, `MORALE`, `FULL`, `SIMPLIFIED` |
| Tags de ação | lowercase kebab-case | `social`, `leisure`, `offensive`, `defensive`, `stealth`, `break-convention` |
| Funções em pseudocódigo | snake_case | `score_action()`, `select_action()`, `update_considerations()` |

A prosa da spec permanece em português; apenas identificadores usam inglês.

---

## 3. Modelo de dados

Faixas e unidades são `[PREMISSA]`; os conceitos estruturais são o núcleo da arquitetura.

### 3.1 `Consideration`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `id` | string/enum | ex.: `HUNGER`, `ENERGY`, `THREAT`, `AMMO`, `COVER`, `BOND(target)`, `FEAR`, `FATIGUE`, `MISSION_PROGRESS` | Conjunto aberto e extensível por jogo. |
| `value` | real | [`min`, `max`] | Convenção: +100 = "ótimo/saciado/seguro"; -100 = colapso. `[PREMISSA]` |
| `min` / `max` | real | default -100 / +100 | Limites do clamp (§4.2) e da saturação (§6.2). Considerações unipolares usam `min` = 0 quando não há "excesso" (ex.: `THREAT`). `[PREMISSA]` |
| `updater` | ref. para `ConsiderationUpdater` | §4 | Como o valor evolui (decaimento, evento, agregação espacial…). |
| `base_weight` | real | [0, 10] | Prioridade relativa intrínseca (sobrevivência > conforto). |
| `critical_threshold` | real | [-100, 0] | Abaixo dele, a consideração domina a decisão. |
| `response_curve` | ref. para curva de resposta | §4.3 | Converte valor → pressão. |

**Propriedade essencial:** o motor de decisão trata todas as considerações de forma uniforme. `HUNGER` que decai com o tempo, `THREAT` derivada de linha de visão inimiga e `AMMO` lida do inventário são apenas atualizadores diferentes alimentando o mesmo pipeline.

### 3.2 `Trait` e `Personality`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `Trait.id` | string | ex.: `lazy`, `cautious`, `sadistic`, `loyal`, `inappropriate` | Conjunto extensível. |
| `Trait.modifiers` | mapa (tag de ação → multiplicador) | multiplicador ∈ [0, 5] | Ex.: `cautious` ×1,5 em ações com tag `defensive`. |
| `Trait.consideration_delta` | mapa (consideração → ajuste de atualizador) | depende do updater | Traços podem criar ou modular considerações, não só filtrar ações. Ex.: `paranoid` aumenta a taxa de acúmulo de `FEAR`. `[PREMISSA]` |
| `Personality.preferences` | mapa (domínio → valor) | [-10, +10] | "Do que o agente gosta e não gosta". Ex.: erudito → +livro, −arena. |
| `Personality.traits` | lista de `Trait` | 0..N | `[PREMISSA]`: N típico = 3–5. |

### 3.3 `Agent`

| Campo | Tipo | Notas |
|---|---|---|
| `id` | uuid | |
| `considerations` | mapa (id → `Consideration`) | §3.1 |
| `personality` | `Personality` | §3.2 |
| `action_queue` | `ActionQueue` (lista ordenada de `ActionInstance`) | §8; ações em andamento e enfileiradas. |
| `current_action` | `ActionInstance` ou nulo | |
| `position` | coordenada no mundo | Usada no termo de distância (§6). |
| `capabilities` | conjunto (skills, facção, classe, equipamento) | Alimenta precondições de anúncios (§5.2). |
| `active_contexts` | lista de `Context` | Modificadores situacionais temporários (§6.4). |
| `simulation_level` | enum {`FULL`, `SIMPLIFIED`} | LOD, §8.4. |
| `narrative_commitments` | lista de `NarrativeCommitment` | Estado que a autonomia não deve destruir. |

### 3.4 `AffordanceProvider`

| Campo | Tipo | Notas |
|---|---|---|
| `id` | uuid | |
| `position` | coordenada ou região | Provedores podem ser pontuais (item), extensos (zona de cobertura) ou móveis (outro NPC). |
| `advertised_actions` | lista de `AdvertisedAction` | O coração do protocolo (§5). |
| `capacity` | inteiro ≥ 0 | Quantos agentes podem usá-lo simultaneamente. `[PREMISSA]` |
| `occupants` | lista de agentes | |
| `state` | opaco | Anúncios podem ser condicionais ao estado (arma sem munição não anuncia `shoot`). |

**Observações:**
- Um agente é também um `AffordanceProvider`: NPCs anunciam conversa, comércio, duelo, recrutamento.
- Provedores abrangem: objetos, locais, zonas (cobertura, patrulha, perigo), eventos (alarme, festival), itens de inventário e outros atores.

### 3.5 `AdvertisedAction`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `action_id` | string | ex.: `eat`, `reload`, `flank`, `converse`, `harvest` | |
| `deltas` | mapa (consideração → real) | [-100, +100] por consideração | Quanto a ação **promete** alterar cada consideração por execução completa. |
| `estimated_duration` | real | segundos/minutos de jogo, > 0 | `[PREMISSA]` |
| `tags` | conjunto de strings | ex.: {`social`, `combat`, `defensive`, `work`, `mischief`} | Alvo dos modificadores de traço/contexto. `[PREMISSA]` |
| `domain` | string | ex.: `book`, `arena` | Domínio de preferência consultado em `Personality.preferences` via `preference(A, domain(x))` (§6.2). `[PREMISSA]` |
| `preconditions` | lista de predicados sobre o agente | | Skill, facção, estado, capacidade livre, posse de item. Ver §5.2. |
| `intrinsic_priority` | real | [0, 10] | Algumas ações têm prioridade estrutural (ex.: ordens diretas, reações de sobrevivência). |
| `advertisement_radius` | real | metros de jogo | Alcance em que o anúncio é visível. `[PREMISSA]` |
| `cost` | mapa (recurso → quantidade) | ≥ 0 | Ações podem consumir recursos (munição, stamina, dinheiro). `[PREMISSA]` |
| `capacity` | inteiro ≥ 0 | Quantos agentes podem executar **esta ação** simultaneamente. **0 = sem limite** — deliberadamente diferente de `AffordanceProvider.capacity`, onde 0 bloqueia. O motivo e as consequências estão em §5.4. |
| `occupancy` | inteiro ≥ 0 | Quantos agentes já executam esta ação no início do tick, informado pelo cliente; consome vagas da ação antes de qualquer decisão nova (§5.4). |

---

## 4. Atualizadores de consideração

O decaimento temporal de necessidades fisiológicas é **um** atualizador. O modelo admite vários tipos, combináveis por consideração `[PREMISSA]`:

### 4.1 Tipos de atualizador

| Tipo | Dinâmica | Exemplos de consideração |
|---|---|---|
| **Decaimento linear** (`linear_decay`) | `value -= decay_rate * Δt` | `HUNGER`, `ENERGY`, `HYGIENE`, `FATIGUE` |
| **Regeneração linear** (`linear_regen`) | `value += regen_rate * Δt` | `HEALTH` fora de combate, `STAMINA` em repouso |
| **Dirigido por evento** (`event_driven`) | alterado apenas por ações/eventos (`deltas`, dano, coleta) | `AMMO`, `MONEY`, `MISSION_PROGRESS` |
| **Dirigido por percepção** (`perception_driven`) | função contínua do ambiente sensado (linha de visão, distância de inimigos, exposição) | `THREAT`, `COVER`, `VISIBILITY` |
| **Dirigido por relação** (`relationship_driven`) | evolui por interações com um ator específico | `BOND(target)`, `TRUST(faction)` |
| **Agregado de contexto** (`context_aggregate`) | função do local (lotação, decoração, perigo da zona) | `ENVIRONMENT`, `GROUP_MORALE` |

### 4.2 Atualização por tick

```
para cada consideration c do agente:
    c.value = apply(c.updater, c, agent, world, Δt)
    c.value = clamp(c.value, c.min, c.max)
    aplicar efeitos de traits sobre o updater (Trait.consideration_delta)
```

Exemplo de configuração de decaimento `[PREMISSA]` (ciclo de referência: 24h de jogo ≈ dia completo; calibrado para 2–3 refeições/dia e 1 descanso longo/dia):

| Consideração | Updater | Taxa (pts/h) | Tempo +100 → -100 | `critical_threshold` |
|---|---|---|---|---|
| `HUNGER` | `linear_decay` | 10 | 20 h | -50 |
| `ENERGY` | `linear_decay` | 12,5 | 16 h | -60 |
| `FATIGUE` (combate) | `linear_decay` em combate; `linear_regen` em repouso | 20 / -30 | 10 h | -40 |
| `HYGIENE` | `linear_decay` | 6 | ~33 h | -30 |
| `THREAT` | `perception_driven` | — | instantâneo | -40 |
| `AMMO` | `event_driven` | — | — | 10 (unipolar 0..100) |
| `ENVIRONMENT` | `context_aggregate` | — | — | -10 |

### 4.3 Curvas de resposta (valor → pressão)

A pressão (`pressure`) é o peso da consideração na utilidade. Forma geral:

```
pressure(c) = c.base_weight * response_curve(c.value)
```

Curvas úteis `[PREMISSA]`:

| Curva | Fórmula | Uso típico |
|---|---|---|
| Convexa (urgência) | `((100 - v)/200)^response_curve_exponent`, default = 2 | Necessidades críticas: o peso dispara perto do fundo. Com `response_curve_exponent` = 2, v = -80 pesa ~4× mais que v = -20. |
| Linear | `(100 - v)/200` | Recursos com pressão proporcional (`AMMO`). |
| Degrau | `1 se v < threshold, senão k` | Alarmes binários, estados de missão. |
| Logística | `1 / (1 + e^(-a(v - b)))` | Transições suaves com ponto de inflexão calibrável (`FEAR`, `MORALE`). |

**Saturação:** o ganho de uma ação é limitado pelo teto da consideração — executar ação sobre consideração já alta rende pouco (quem acabou de descansar não valoriza a cama). Isso emerge da função de utilidade (§6.2), não de regra especial.

**Colapso:** valor no mínimo sustentado dispara consequências (desmaio, pânico, quebra de moral, fome debilitante). As consequências específicas são definidas por jogo. `[PREMISSA]`

---

## 5. Protocolo de anúncio de affordances

### 5.1 Ciclo de vida do anúncio

1. **Registro:** ao ser instanciado/ativado no mundo, o provedor publica sua lista de `AdvertisedAction` num índice espacial consultável (§8.2). `[PREMISSA]` (mecanismo de descoberta)
2. **Descoberta:** quando o agente entra em estado de decisão (§8.3), coleta todos os anúncios cujo `advertisement_radius` cobre sua posição e cujas `preconditions` ele satisfaz.
3. **Avaliação:** cada anúncio elegível recebe uma pontuação (§6).
4. **Reserva:** ao escolher, o agente ocupa uma vaga do provedor (`capacity`) **e** uma vaga da ação (`capacity` da `AdvertisedAction`, §5.4); anúncios sem vaga livre em qualquer um dos dois níveis deixam de ser elegíveis para outros agentes. Disputas dentro de um mesmo lote de decisão são arbitradas pelo serviço (§6.5).
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

## 6. Algoritmo de seleção de ação

### 6.1 Visão geral

O agente pontua cada ação anunciada ao alcance combinando: (a) quanto a ação atende às considerações prementes, (b) prioridade intrínseca, (c) personalidade/traços, (d) contexto situacional, (e) distância/custo de acesso. Depois aplica seleção estocástica (§7). A pontuação muda o tempo inteiro, porque as considerações mudam o tempo inteiro.

### 6.2 Função de utilidade

Para agente `A` e anúncio `x` (ação oferecida pelo provedor `P`):

```
score_action(A, x) = [ Σ_c  pressure(c) * expected_gain(c, x) ]
                + x.intrinsic_priority * W_PRIORITY
                * M_personality(A, x)
                * M_context(A, x)
                * M_distance(A, P)
                - weighted_cost(A, x)
```

Componentes:

```
expected_gain(c, x) = min( x.deltas[c], c.max - c.value )       // saturação: promessa além do teto não vale
pressure(c)         = c.base_weight * response_curve(c.value)   // §4.3
M_personality       = Π_t∈A.personality.traits  t.modifiers[tags(x)]  *  (1 + preference(A, domain(x)) / 10)
M_context           = Π_k∈A.active_contexts  k.modifiers[tags(x)]
M_distance          = 1 / (1 + distance(A, P) / DISTANCE_REFERENCE)   // DISTANCE_REFERENCE ≈ 10 m de jogo [PREMISSA]
weighted_cost       = Σ_r  x.cost[r] * scarcity(A, r)                 // recursos escassos pesam mais [PREMISSA]
```

- **Distância** entra como decaimento hiperbólico `[PREMISSA]`; pode ser substituída por custo de pathfinding real quando disponível.
- **Personalidade** é multiplicativa sobre tags: o NPC erudito tem `preference(book) > 0`; o `sadistic` tem multiplicador > 1 em tags {`intimidate`, `torture`}.
- **Urgência** entra via `pressure`: curvas convexas fazem uma consideração crítica dominar a soma (sob fogo pesado, `THREAT` esmaga `FUN`).
- **Custos** tornam ações caras menos atraentes conforme o recurso escasseia.

### 6.3 Pseudocódigo (neutro de linguagem)

```
funcao select_action(agent A) -> ActionInstance ou nulo:
    candidates = []
    para cada provider P em advertised_actions_in_range(A.position):
        para cada action x em P.active_advertisements(A):          // preconditions + estado do provedor
            se P.capacity - |P.occupants| <= 0: continuar
            se x.capacity > 0 e x.occupancy >= x.capacity: continuar   // §5.4: ação sem vaga
            se nao can_afford(A, x.cost): continuar
            u = score_action(A, x)                                  // §6.2
            candidates.adicionar((x, P, u))

    se candidates vazio: retornar default_idle_action(A)

    ordenar candidates por u decrescente
    top = primeiros SELECTION_TOP_K candidates                      // SELECTION_TOP_K = 3 [PREMISSA]

    chosen = sample_softmax(top, SELECTION_TEMPERATURE)             // §7.1
    retornar instantiate(chosen, A)
```

### 6.4 Contextos situacionais

Ao entrar num local, assumir um papel ou mudar de estado de missão, o agente recebe um `Context { modifiers: mapa(tag → real), duration }`. Contextos são o mecanismo pelo qual o ambiente "sugere" comportamento sem codificar regras no agente. Exemplos `[PREMISSA]`:

| Contexto | Efeito |
|---|---|
| `location:gym` | ×2,0 em tag `exercise` |
| `role:host` | ×1,5 em `social`, ×1,3 em `serve` |
| `role:guest` | ×1,3 em `social`, ×0,7 em `invasive` |
| `state:under_fire` | ×3,0 em `defensive`, ×0,3 em `social` |
| `mission:escort` | ×2,5 em `protect-target` |
| `trait:inappropriate` | ×2,0 em `break-convention` (inversão deliberada de incentivos sociais) |

### 6.5 Arbitragem de disputa dentro do lote

Quando as decisões são produzidas em lote (vários agentes de uma vez, como no `BatchDecide` da implementação de referência), dois agentes podem pontuar e escolher o mesmo recurso sem ver a decisão um do outro. A arbitragem é do serviço, não do cliente: **nenhum lote devolve mais agentes para um provedor ou para uma ação do que eles têm vagas** (§5.4).

**Critério: quem está mais perto leva.** A lógica é física, não de mérito — quem alcança o recurso antes fica com ele. Utilidade **não** é critério de desempate: "quem tem mais fome come primeiro" não existe no mundo real, fome não acelera ninguém. Registrar explicitamente, porque "maior utilidade ganha a disputa" é a escolha intuitiva que será proposta de novo — e está errada: ela faria o NPC mais necessitado teleportar prioridade para si, destruindo a legibilidade espacial da simulação.

**Mecânica.** A pontuação continua paralela (cada agente é pontuado independentemente); só a reconciliação é serial:

1. Cada agente produz sua **lista ordenada de preferência**: a primeira posição é a escolha **estocástica** usual (softmax sobre os `SELECTION_TOP_K` melhores, §7) — a imperfeição deliberada sobrevive à arbitragem, a arbitragem não vira argmax. As posições seguintes são os demais candidatos em ordem decrescente de utilidade, servindo apenas de plano B. A lista é limitada a `RECONCILIATION_TOP_K` candidatos (default 5): em lotes de dezenas de milhares de agentes, reter a lista completa por agente estoura memória.
2. A reconciliação distribui as vagas de cada provedor disputado aos contendores **mais próximos**, respeitando os dois níveis de capacidade. Quem perde cai para o próximo candidato da **própria** lista — o que pode criar disputa nova em outro provedor, então o processo é **iterativo** e repete até estabilizar.
3. Agente que esgotar os candidatos sai sem ação (na implementação de referência, `selected_action_index = -1`), como no caso de nenhum anúncio elegível.

**Terminação garantida:** o cursor de um agente na própria lista só anda para frente — desalojado do candidato `i`, ele nunca mais aponta para `i`. Cada agente é desalojado de cada candidato no máximo uma vez, logo o total de desalojamentos é limitado pela soma dos tamanhos das listas (finita). Não há laço infinito.

**Determinismo inegociável:** a ordem da reconciliação é explícita e estável — distância crescente, desempate por ID do agente. Nunca depende de ordem de iteração de mapa nem de ordem de conclusão de goroutine: mesma entrada + mesma seed = mesma atribuição, inclusive nas disputas.

**Statelessness preservada:** a arbitragem acontece inteiramente dentro de uma chamada; o serviço não guarda mundo entre requisições.

---

## 7. Fontes deliberadas de imperfeição e como sintonizá-las

O agente não deve escolher sempre a melhor opção. O erro faz parte da ilusão de vida e protege o jogo contra exploração pelo jogador.

| # | Fonte de imperfeição | Mecanismo | Parâmetro de sintonia | Efeito de aumentar |
|---|---|---|---|---|
| 1 | **Seleção estocástica** | Softmax sobre os `SELECTION_TOP_K` melhores candidatos: `P(i) = e^(u_i / SELECTION_TEMPERATURE) / Σ_j e^(u_j / SELECTION_TEMPERATURE)` | `SELECTION_TEMPERATURE` (default 1,0) e `SELECTION_TOP_K` (default 3) `[PREMISSA]` | `SELECTION_TEMPERATURE`↑ → escolhas mais aleatórias; →0 → argmax (proibido em produção). `SELECTION_TOP_K`↑ → mais variedade. |
| 2 | **Compromisso com ação subótima** | Uma vez iniciada, a ação só é interrompida por urgência alta (§8.3), mesmo se outra opção a superar | `preemption_margin` (default 1,5×) `[PREMISSA]` | Margem↑ → agente "teimoso", cenas cômicas/dramáticas; margem↓ → agente reativo/robótico. |
| 3 | **Sem otimização global** | O agente só vê anúncios ao alcance; não planeja rotas nem agenda de longo prazo | `advertisement_radius` por provedor | Raio↑ → decisões mais "espertas", menos deambulação. |
| 4 | **Convenções imperfeitas** | Não implementar (ou probabilisticamente ignorar) regras sociais/táticas "corretas" | `convention_break_probability` (default 0,15) `[PREMISSA]` | ↑ → mais situações estranhas/engraçadas/memoráveis. |
| 5 | **Traços que invertem incentivos** | Traços como `inappropriate` ou `reckless` bonificam o comportamento "errado" | multiplicadores do traço | ↑ → mais gafes, riscos absurdos, drama. |
| 6 | **Percepção ruidosa** `[PREMISSA]` | Considerações `perception_driven` recebem ruído ou atraso de atualização | `perception_noise` (default σ = 5 pts) | ↑ → erros de leitura do mundo (NPC não percebe a ameaça óbvia). |

**Diretriz de tuning:** o alvo não é minimizar erro, é maximizar **variedade legível**: o observador deve conseguir contar uma história sobre por que o agente fez aquilo.

---

## 8. Ciclo de execução

### 8.1 Loop de atualização (por tick, por agente em nível `FULL`)

```
1. update_considerations(A, Δt)                        // §4 (todos os updaters)
2. se A.current_action != nulo:
       apply_gradual_deltas(A.current_action, Δt)
       se is_action_complete(A.current_action): A.current_action = nulo
3. se A.current_action == nulo e action_queue vazia:
       a = select_action(A)                             // §6.3
       se a: start_action(a)
4. senao: check_preemption(A)                           // §8.3
5. process_queue(A)                                     // §8.2
```

### 8.2 Fila de ações (`ActionQueue`)

- O jogador (ou diretor de jogo, ou script de missão) pode enfileirar ações; elas têm prioridade sobre a autonomia, **mas** considerações críticas podem interrompê-las.
- `NarrativeCommitment` (ex.: aliança, romance, missão escoltada) bloqueia ações autônomas que o contradigam por uma janela de tempo. `[PREMISSA]`

### 8.3 Interrupção / preempção

```
funcao check_preemption(A):
    current = A.current_action
    se current == nulo: retornar
    se existe consideration c com c.value < c.critical_threshold:
        best = best_candidate_for(c)                     // reavalia anúncios
        se best e score_action(A, best) > preemption_margin * continuation_utility(current):
            interrupt(current)                           // deltas parciais já aplicados se mantêm
            start_action(best)
```

Ações enfileiradas pelo jogador só são preemptadas por colapso iminente (`value` ≤ -90). `[PREMISSA]`

### 8.4 Níveis de detalhe de simulação (LOD)

Agentes longe do jogador são simulados de forma simplificada (eventos agregados: trabalhou, viajou, fez inimigo, foi derrotado); ao retornar à vizinhança do jogador, o estado é **reconstruído de forma plausível** (quem acabou de jantar não pode reaparecer faminto; quem estava em batalha pode reaparecer ferido).

- **`FULL`:** loop de §8.1, tick fino (ex.: 1 s–1 min de jogo). `[PREMISSA]`
- **`SIMPLIFIED`:** tick grosso (ex.: 1 h de jogo); considerações avançam por modelo agregado; decisões são eventos discretos. `[PREMISSA]`
- **Transição `SIMPLIFIED` → `FULL`:** inicializar considerações com valores condizentes com a agenda simulada e a hora do dia. `[PREMISSA]` no mecanismo.

---

## 9. Instanciação por gênero

A mesma maquinaria (`considerations → response curves → utility → stochastic selection`) configurada para quatro casos distintos. Nada muda no motor; mudam as considerações, os provedores, as tags e o tuning.

### 9.1 Simulação social / vida

| Elemento | Configuração |
|---|---|
| Considerações | `HUNGER`, `ENERGY`, `HYGIENE`, `BLADDER`, `FUN`, `COMFORT`, `SOCIAL`, `ENVIRONMENT` (`linear_decay`, curva convexa `response_curve_exponent`=2) |
| Provedores | Mobília e eletrodomésticos, locais de lazer, outros NPCs (socialização), eventos (festas) |
| Tags típicas | `social`, `leisure`, `domestic`, `mischief`, `romance` |
| Traços exemplares | `lazy` (×1,5 em `sit`), `erudite` (pref livro+), `mean` (×2 em `provoke`), `inappropriate` (×2 em `break-convention`) |
| Tuning | `SELECTION_TEMPERATURE` alto (1,0–1,5), `preemption_margin` alta (1,5–2,0), `convention_break_probability` 0,15 — máxima emergência narrativa; `W_PRIORITY` baixo. |

### 9.2 Combate tático / stealth

| Elemento | Configuração |
|---|---|
| Considerações | `THREAT` (`perception_driven`, logística), `COVER` (`perception_driven` espacial), `AMMO` (`event_driven`, linear), `FATIGUE` (`linear_decay` em combate), `VISIBILITY` (`perception_driven` — quanto o inimigo me vê), `MORALE` (`context_aggregate` de grupo) |
| Provedores | Zonas de cobertura, pontos de flanco, posições elevadas, corpos saqueáveis, alarmes, rotas de patrulha, aliados (anunciam `heal`, `cover`) |
| Tags típicas | `offensive`, `defensive`, `stealth`, `support`, `retreat` |
| Traços exemplares | `cautious` (×2 em `defensive`), `reckless` (×2 em `offensive`, inversão deliberada), `loyal` (×2 em `protect-ally`) |
| Tuning | `SELECTION_TEMPERATURE` baixo (0,3–0,6) — competência tática vende o gênero; `preemption_margin` baixa (1,1–1,3) para reação rápida a `THREAT`; `perception_noise` > 0 em stealth para erros de detecção críveis. Imperfeição migra da seleção para a **percepção**. |

### 9.3 Survival / crafting

| Elemento | Configuração |
|---|---|
| Considerações | `HUNGER`, `THIRST`, `COLD`/`HEAT` (`perception_driven` ambiental), `INJURY` (`event_driven`), `ENERGY`, `STOCK(resource)` (`event_driven`), `SHELTER_SAFETY` (`context_aggregate`) |
| Provedores | Recursos coletáveis (planta, minério, caça), estações de crafting, fogueira, abrigo, fontes de água, clima (evento que anuncia `seek_shelter`) |
| Tags típicas | `gathering`, `construction`, `cooking`, `hunting`, `rest` |
| Traços exemplares | `hardworking` (×1,5 em `gathering`), `gluttonous` (×1,5 em `cooking`), `nomadic` (×0,7 em `construction`) |
| Tuning | `SELECTION_TEMPERATURE` médio (0,7–1,0); custos de recursos com peso alto (escassez dói); `critical_threshold` de `HUNGER`/`COLD` agressivos para forçar o loop de sobrevivência; `advertisement_radius` grande para recursos (exploração) e pequeno para estações. |

### 9.4 NPC de vila / ambiente em RPG de mundo aberto

| Elemento | Configuração |
|---|---|
| Considerações | Rotina diária modelada como `ENERGY`/`HUNGER`/`DUTY` (`linear_decay`), `SECURITY` (`perception_driven` de crime/perigo), `BOND(player)` (`relationship_driven`), `VILLAGE_PROSPERITY` (`context_aggregate`), `CURIOSITY` (`linear_decay` lento — atrai para eventos) |
| Provedores | Lojas e postos de trabalho (anunciam `work`), taverna, templo, casa, praça, o próprio jogador (anuncia `converse`, `trade`), eventos dinâmicos (incêndio, festival, ataque) |
| Tags típicas | `work`, `social`, `devotion`, `commerce`, `flee` |
| Traços exemplares | `devout` (×2 em `devotion`), `greedy` (×1,5 em `commerce`), `cowardly` (×3 em `flee` quando `SECURITY` crítica) |
| Tuning | `SELECTION_TEMPERATURE` alto (1,0–1,5) para vilas vivas e imprevisíveis; LOD agressivo (a maioria dos NPCs roda em `SIMPLIFIED`); `narrative_commitments` fortes (NPC de missão não abandona o posto por `HUNGER` não-crítica). |

---

## 10. Parâmetros de tuning consolidados

Todos os valores default são `[PREMISSA]`.

| Parâmetro | Default | Faixa | Seção |
|---|---|---|---|
| Faixa de consideração (`value`) | [-100, +100] (ou [0, 100] unipolar) | fixo | 3.1 |
| `response_curve_exponent` (convexidade da urgência) | 2,0 | [1, 4] | 4.3 |
| `decay_rate` por consideração | ver §4.2 | 3–20 pts/h | 4.2 |
| `critical_threshold` por consideração | ver §4.2 | [-90, -10] | 4.2 |
| `W_PRIORITY` (peso da prioridade intrínseca) | 5 | [0, 20] | 6.2 |
| `DISTANCE_REFERENCE` (distância de referência) | 10 m | [2, 50] | 6.2 |
| `SELECTION_TOP_K` (candidatos na seleção) | 3 | [1, 10] | 6.3 |
| `SELECTION_TEMPERATURE` (temperatura softmax) | 1,0 (0,3–1,5 por gênero, §9) | [0,1, 5] | 7.1 |
| `preemption_margin` | 1,5× | [1,0, 3,0] | 7.2 / 8.3 |
| `convention_break_probability` | 0,15 | [0, 1] | 7.4 |
| `perception_noise` (σ) | 5 pts | [0, 30] | 7.6 |
| Multiplicadores de traço (`Trait.modifiers`) | 0,5–2,0 | [0, 5] | 3.2 / 6.4 |
| Tick `FULL` / `SIMPLIFIED` | 1 min / 1 h de jogo | — | 8.4 |
| `capacity` default de provedor | 1 | [0, ∞) | 3.4 |
| `capacity` de ação | 0 (**sem limite** — §5.4) | [0, ∞) | 3.5 / 5.4 |
| `RECONCILIATION_TOP_K` (candidatos retidos por agente) | 5 | [1, ∞) | 6.5 |
| `advertisement_radius` default | mesmo cômodo/zona (~8 m) | [1, 100] m | 3.5 |

---

## 11. Critérios de aceitação (observáveis e testáveis)

1. **CA-1 (Atualizadores):** com nenhuma ação executada, uma consideração `linear_decay` configurada com `decay_rate` = 10 pts/h vai de +100 a -100 em ~20 h de jogo (±10%). Considerações `event_driven` não se movem sem eventos; considerações `perception_driven` respondem a mudanças do ambiente em ≤ 1 tick.
2. **CA-2 (Publicidade / extensibilidade):** instanciar um provedor novo com anúncio `{rest: ENERGY +80}` em ambiente previamente sem fonte de `ENERGY` faz agentes com `ENERGY` < `critical_threshold` usá-lo **sem nenhuma alteração no código do agente**.
3. **CA-3 (Urgência domina):** agente com `HUNGER` = -70 e `FUN` = -10, diante de provedores de comida e lazer equidistantes com deltas equivalentes, escolhe comer em ≥ 90% de 200 amostragens (com `SELECTION_TEMPERATURE` default).
4. **CA-4 (Saturação):** agente com `ENERGY` = +95 nunca escolhe `rest` quando existe qualquer outra ação com utilidade > 0 disponível.
5. **CA-5 (Personalidade):** dois agentes idênticos exceto `preference(book)` = +8 vs `preference(arena)` = +8, com `FUN` = -50 e ambos os provedores disponíveis, divergem na escolha em ≥ 80% das amostragens.
6. **CA-6 (Imperfeição):** em 1.000 decisões com um candidato de utilidade destacada (2× o segundo), a melhor opção é escolhida em 60–90% dos casos — nunca 100% (`SELECTION_TEMPERATURE` > 0 garantido) e nunca ≤ 1/`SELECTION_TOP_K` (não é aleatório puro).
7. **CA-7 (Distância):** duplicar a distância de um provedor reduz sua utilidade pelo fator previsto por `M_distance` (§6.2), mantidos os demais termos.
8. **CA-8 (Precondições):** ação cuja `preconditions` (skill, facção, item, estado) não é satisfeita jamais aparece entre os candidatos pontuados; ao ganhar a skill/facção, o anúncio passa a ser elegível sem reiniciar o agente.
9. **CA-9 (Contexto):** agente que entra num local com `Context` ×2,0 em tag `exercise` aumenta a frequência de ações dessa tag em ≥ 2× durante a permanência, vs. baseline fora dele.
10. **CA-10 (Preempção):** agente executando ação de lazer com uma consideração cruzando o `critical_threshold` interrompe a ação e busca provedor que a atenda em ≤ 1 tick de decisão.
11. **CA-11 (LOD):** agente em `SIMPLIFIED` cuja agenda agregada incluiu "refeição" às 12h e retorna ao modo `FULL` às 13h reaparece com `HUNGER` ≥ +30 (estado plausível, não faminto).
12. **CA-12 (Compromisso narrativo):** durante um `NarrativeCommitment` ativo (ex.: romance, escolta), o agente não inicia autonomamente ações que o contradigam.
13. **CA-13 (Generalidade):** o mesmo motor, sem alteração de código, executa as quatro configurações de §9 apenas trocando tabelas de considerações, provedores, tags e parâmetros.
14. **CA-14 (Deltas mistos):** ação com deltas {+`FUN` 40, −`ENERGY` 30} é escolhida por agente entediado e descansado, e evitada por agente exausto e entediado, em ≥ 80% das amostragens de cada caso.
15. **CA-15 (Arbitragem por proximidade):** dois agentes disputando um provedor de capacidade 1 no mesmo lote: exatamente um é atribuído, e é o mais próximo — mesmo quando o mais distante tem utilidade estritamente maior (ex.: `HUNGER` -95 longe vs. `HUNGER` -10 perto; o faminto perde).
16. **CA-16 (Plano B após derrota):** o perdedor de uma disputa recebe o próximo candidato da própria lista de preferência; esgotada a lista, sai sem ação (`-1`).
17. **CA-17 (Capacidade maior que 1):** provedor com N vagas livres recebe exatamente os N contendores mais próximos; os demais caem para plano B ou `-1`.
18. **CA-18 (Cascata e estabilização):** a queda de um agente para seu plano B pode desalojar um agente mais distante já posicionado ali; a reconciliação itera até estabilizar, e o resultado final respeita todas as vagas.
19. **CA-19 (Determinismo da arbitragem):** mesma entrada e mesma seed produzem exatamente a mesma atribuição, inclusive com disputas nos dois níveis de capacidade.
20. **CA-20 (Invariante de vagas do provedor):** em nenhum resultado a contagem de agentes atribuídos a um provedor excede `capacity - |occupants|` — inclusive quando o cliente reporta ocupação parcial.
21. **CA-21 (Primeira preferência estocástica):** a arbitragem não vira argmax: a primeira preferência de cada agente é idêntica à que a seleção estocástica (§7) produziria sem arbitragem, dada a mesma seed.
22. **CA-22 (Capacidade por ação):** ação com `capacity` C admite no máximo C agentes por lote, mesmo com vagas sobrando no provedor (bancada com 4 vagas e serra com 1: exatamente 1 serra, no máximo 4 no total).
23. **CA-23 (Zero é ilimitado):** ação com `capacity` 0 — ou campo ausente na requisição — não impõe limite próprio e jamais bloqueia candidatos; vale apenas o limite do provedor.
24. **CA-24 (O mais restritivo manda):** ação com `capacity` 3 num provedor com `capacity` 1 admite exatamente 1 agente.
25. **CA-25 (Ocupação por ação):** `occupancy` informada pelo cliente consome vagas da ação antes do lote: ação com `capacity` 2 e `occupancy` 1 admite só mais 1 agente; ação com `occupancy` = `capacity` não é elegível para ninguém naquele tick.
26. **CA-26 (Invariante dos dois níveis):** em nenhum resultado a contagem por ação excede a `capacity` da ação (quando > 0), nem a contagem por provedor excede a do provedor.
27. **CA-27 (Candidatos retidos):** a lista de preferência retida por agente tem no máximo `RECONCILIATION_TOP_K` entradas — a escolha estocástica seguida dos fallbacks em ordem decrescente de utilidade; `RECONCILIATION_TOP_K` = 0 no tuning usa o default (5).

---

## Apêndice A — Proveniência e limites desta especificação

**Origem.** Esta especificação nasceu da análise de um vídeo divulgativo (canal Vertex, pt-BR, ~15 min) sobre a inteligência artificial de um jogo de simulação de vida, transcrita automaticamente pelo YouTube. O vídeo descrevia, em linguagem informal: necessidades que decaem, objetos que anunciam o que oferecem, seleção por pontuação com aleatoriedade deliberada, traços de personalidade que criam motivações, e simulação em dois níveis de detalhe.

**Generalização.** A versão original desta spec era específica daquele domínio; esta versão abstrai o núcleo (`considerations → response curves → utility → selection`; provedores de affordance) para servir a qualquer gênero de jogo. Necessidades fisiológicas com decaimento são aqui apenas uma instanciação entre várias (§4.1, §9).

**O que este documento NÃO é.** Nada aqui é afirmação sobre o código, parâmetros ou arquitetura reais daquele jogo ou de seu estúdio. Todos os números, fórmulas, mecanismos de seleção e estruturas de dados são decisões de engenharia desta spec (`[PREMISSA]`), plausíveis mas inventadas.

**Versionamento.** Este documento viveu originalmente fora de qualquer repositório. A partir da versão que introduz §5.4 e §6.5, ele passa a ser versionado junto do código no repositório do geppetto (`docs/spec.md`), implementação de referência desta arquitetura — a cópia no repositório é a canônica. As seções novas descrevem comportamento já implementado e testado (CA-15 em diante), não aspiração.

**Notas de interpretação da transcrição** (legendas automáticas; trechos possivelmente corrompidos, interpretados pelo contexto):

1. **"Simless" → "Simlish"**: idioma inventado dos personagens; o contexto (fala ambígua, emoção sem significado exato) confirma. O conceito subjacente — ambiguidade deliberada como espaço para a imaginação do jogador — sobrevive na diretriz de "variedade legível" (§7).
2. **"Sim and" → "SimAnt"**: jogo anterior do mesmo criador, sobre formigas atraídas por feromônios; citado como origem histórica da ideia de agentes atraídos por sinais do ambiente — análoga ao protocolo de anúncios (§5).
3. **"Anúncios"**: mantido do original; corresponde ao conceito conhecido de *advertisement* em smart objects, aqui generalizado para affordances (`AdvertisedAction`).
4. **"IA utilitária"**: o vídeo usa o termo corretamente (*utility AI*); esta spec o adota como nome da arquitetura.
