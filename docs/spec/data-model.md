# Modelo de dados e atualizadores de consideração

## 3. Modelo de dados

Faixas e unidades são `[PREMISSA]`; os conceitos estruturais são o núcleo da arquitetura.

### 3.1 `Consideration`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `id` | string/enum | ex.: `HUNGER`, `ENERGY`, `THREAT`, `AMMO`, `COVER`, `BOND(target)`, `FEAR`, `FATIGUE`, `MISSION_PROGRESS` | Conjunto aberto e extensível por jogo. |
| `value` | real | [`min`, `max`] | Convenção: +100 = "ótimo/saciado/seguro"; -100 = colapso. `[PREMISSA]` |
| `min` / `max` | real | default -100 / +100 | Limites do clamp ([§4.2](data-model.md#42-atualização-por-tick)) e da saturação ([§6.2](selection.md#62-função-de-utilidade)). Considerações unipolares usam `min` = 0 quando não há "excesso" (ex.: `THREAT`). `[PREMISSA]` |
| `updater` | ref. para `ConsiderationUpdater` | [§4](data-model.md#4-atualizadores-de-consideração) | Como o valor evolui (decaimento, evento, agregação espacial…). |
| `base_weight` | real | [0, 10] | Prioridade relativa intrínseca (sobrevivência > conforto). |
| `critical_threshold` | real | [-100, 0] | Abaixo dele, a consideração domina a decisão. |
| `response_curve` | ref. para curva de resposta | [§4.3](data-model.md#43-curvas-de-resposta-valor-pressão) | Converte valor → pressão. |

**Ausência de valores e extremos.** O motor não inventa defaults para `min`/`max`: eles fazem parte da declaração da consideração. Uma consideração declarada sem limites (ou com `max` ≤ `min`) fica **inerte** — a curva de resposta não tem amplitude para normalizar e a pressão resultante é sempre 0, ou seja, ela nunca influencia decisão nenhuma, sem erro e sem aviso. A convenção −100/+100 é calibração compartilhada ([§10](tuning.md#10-parâmetros-de-tuning-consolidados)), não uma constante embutida.

`value` fora da faixa não é erro: todo tick termina com clamp em [`min`, `max`] ([§4.2](data-model.md#42-atualização-por-tick)), e um valor inicial fora dos limites é corrigido silenciosamente no primeiro tick. `base_weight` fora de [0, 10] e `critical_threshold` fora de [-100, 0] tampouco quebram nada — mas saem da faixa em que os demais parâmetros foram calibrados.

`critical_threshold` **não participa da pontuação**: quem o consulta é a preempção ([§8.3](execution.md#83-interrupção-preempção)). Uma consideração pode pressionar fortemente a utilidade sem nunca disparar interrupção, e vice-versa.

**Propriedade essencial:** o motor de decisão trata todas as considerações de forma uniforme. `HUNGER` que decai com o tempo, `THREAT` derivada de linha de visão inimiga e `AMMO` lida do inventário são apenas atualizadores diferentes alimentando o mesmo pipeline.

### 3.2 `Trait` e `Personality`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `Trait.id` | string | ex.: `lazy`, `cautious`, `sadistic`, `loyal`, `inappropriate` | Conjunto extensível. |
| `Trait.modifiers` | mapa (tag de ação → multiplicador) | multiplicador ∈ [0, 5] | Ex.: `cautious` ×1,5 em ações com tag `defensive`. |
| `Trait.consideration_delta` | mapa (consideração → ajuste de taxa) | real, aditivo | Traços podem criar ou modular considerações, não só filtrar ações. Ex.: `paranoid` acelera a perda de `FEAR`. `[PREMISSA]` |
| `Personality.preferences` | mapa (domínio → valor) | [-10, +10] | "Do que o agente gosta e não gosta". Ex.: erudito → +livro, −arena. |
| `Personality.traits` | lista de `Trait` | 0..N | `[PREMISSA]`: N típico = 3–5. |

**Como `consideration_delta` atua.** O ajuste é **aditivo** sobre a taxa base do atualizador e acumula ao longo de todos os traços do agente: `taxa_efetiva = taxa_base + Σ_traits delta[consideração]`. Ele só alcança os atualizadores lineares (`linear_decay`, `linear_regen`); os dirigidos por percepção, relação ou contexto não têm taxa e ignoram o ajuste. Um ajuste que zere a taxa congela a consideração; um ajuste que a inverta transforma decaimento em regeneração — ambos permitidos, e às vezes desejáveis (ex.: traço `resilient` com delta negativo em `FATIGUE` reduz o desgaste sem tocá-lo diretamente).

### 3.3 `Agent`

| Campo | Tipo | Notas |
|---|---|---|
| `id` | uuid | |
| `considerations` | mapa (id → `Consideration`) | [§3.1](data-model.md#31-consideration) |
| `personality` | `Personality` | [§3.2](data-model.md#32-trait-e-personality) |
| `action_queue` | `ActionQueue` (lista ordenada de `ActionInstance`) | [§8](execution.md#8-ciclo-de-execução); ações em andamento e enfileiradas. |
| `current_action` | `ActionInstance` ou nulo | |
| `position` | coordenada no mundo | Usada no termo de distância ([§6](selection.md#6-algoritmo-de-seleção-de-ação)). |
| `capabilities` | conjunto (skills, facção, classe, equipamento) | Alimenta precondições de anúncios ([§5.2](affordances.md#52-anúncios-dinâmicos-e-condicionais)). |
| `resources` | mapa (recurso → quantidade ≥ 0) | Estoque consultado pelas precondições de recurso e pelo `cost` dos anúncios ([§3.5](data-model.md#35-advertisedaction)). Recurso ausente do mapa lê-se como 0 — não é erro. |
| `active_contexts` | lista de `Context` | Modificadores situacionais temporários ([§6.4](selection.md#64-contextos-situacionais)). |
| `simulation_level` | enum {`FULL`, `SIMPLIFIED`} | LOD, [§8.4](execution.md#84-níveis-de-detalhe-de-simulação-lod). |
| `narrative_commitments` | lista de `NarrativeCommitment` | Estado que a autonomia não deve destruir. |

### 3.4 `AffordanceProvider`

| Campo | Tipo | Notas |
|---|---|---|
| `id` | uuid | |
| `position` | coordenada ou região | Provedores podem ser pontuais (item), extensos (zona de cobertura) ou móveis (outro NPC). |
| `advertised_actions` | lista de `AdvertisedAction` | O coração do [protocolo (§5)](affordances.md#5-protocolo-de-anúncio-de-affordances). |
| `capacity` | inteiro ≥ 0 | Quantos agentes podem usá-lo simultaneamente. `[PREMISSA]` |
| `occupants` | lista de agentes | |
| `state` | opaco | Anúncios podem ser condicionais ao estado (arma sem munição não anuncia `shoot`). |

**Observações:**
- Um agente é também um `AffordanceProvider`: NPCs anunciam conversa, comércio, duelo, recrutamento.
- Provedores abrangem: objetos, locais, zonas (cobertura, patrulha, perigo), eventos (alarme, festival), itens de inventário e outros atores.
- `occupants` é uma lista de identidades, mas só a **contagem** participa das decisões: as vagas livres do provedor são `capacity − |occupants|`; quem são os ocupantes não influencia a arbitragem.
- `state` é uma string opaca comparada por **igualdade exata** pelas precondições de estado ([§3.5](data-model.md#35-advertisedaction)); o motor não interpreta seu conteúdo.
- No contrato de lote atual, a lista de ocupantes do provedor **não trafega**: o cliente informa apenas `capacity`, e as vagas do provedor são arbitradas contra as decisões do próprio lote. A ocupação que trafega é a por ação (`occupancy`, [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)).

### 3.5 `AdvertisedAction`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `action_id` | string | ex.: `eat`, `reload`, `flank`, `converse`, `harvest` | |
| `deltas` | mapa (consideração → real) | [-100, +100] por consideração | Quanto a ação **promete** alterar cada consideração por execução completa. |
| `estimated_duration` | real | segundos/minutos de jogo, > 0 | `[PREMISSA]` |
| `tags` | conjunto de strings | ex.: {`social`, `combat`, `defensive`, `work`, `mischief`} | Alvo dos modificadores de traço/contexto. `[PREMISSA]` |
| `domain` | string | ex.: `book`, `arena` | Domínio de preferência consultado em `Personality.preferences` via `preference(A, domain(x))` ([§6.2](selection.md#62-função-de-utilidade)). `[PREMISSA]` |
| `preconditions` | lista de predicados | conjunção (E lógico) | Três formas, combináveis: capability exigida (presente no agente), recurso do agente ≥ `minimum`, `required_state` igual ao `state` do provedor. Campo vazio numa precondição não verifica nada. Ver [§5.2](affordances.md#52-anúncios-dinâmicos-e-condicionais) e [§5.5](affordances.md#55-elegibilidade-o-que-é-verificado-antes-e-depois-de-pontuar). |
| `intrinsic_priority` | real | [0, 10] | Algumas ações têm prioridade estrutural (ex.: ordens diretas, reações de sobrevivência). |
| `advertisement_radius` | real | metros de jogo | Alcance em que o anúncio é visível. `[PREMISSA]` |
| `cost` | mapa (recurso → quantidade) | ≥ 0 | Ações podem consumir recursos (munição, stamina, dinheiro). `[PREMISSA]` |
| `capacity` | inteiro ≥ 0 | Quantos agentes podem executar **esta ação** simultaneamente. **0 = sem limite** — deliberadamente diferente de `AffordanceProvider.capacity`, onde 0 bloqueia. O motivo e as consequências estão em [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero). |
| `occupancy` | inteiro ≥ 0 | Quantos agentes já executam esta ação no início do tick, informado pelo cliente; consome vagas da ação antes de qualquer decisão nova ([§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)). |

**Extremos e ausências.** `estimated_duration` deve ser > 0: os `deltas` são entregues à taxa de `delta / estimated_duration` por unidade de tempo ([§5.1](affordances.md#51-ciclo-de-vida-do-anúncio)), e uma duração não positiva torna a ação **estéril** — ela termina sem entregar delta algum. Um delta que nomeia uma consideração que o agente **não possui** é ignorado silenciosamente: não é erro e não bloqueia a ação, o que permite que o mesmo anúncio sirva agentes com conjuntos de considerações diferentes. `cost` sobre um recurso que o agente não tem lê-se como estoque 0, e a ação falha na verificação de pagamento ([§5.5](affordances.md#55-elegibilidade-o-que-é-verificado-antes-e-depois-de-pontuar)).

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

**O que cada tipo consome — e o que acontece quando o dado não vem.**

- `linear_decay` / `linear_regen`: consomem apenas `rate` (pontos por hora de jogo) e Δt. Não dependem de dado externo, e são os únicos alcançados pelo ajuste aditivo de traços ([§3.2](data-model.md#32-trait-e-personality)).
- `event_driven`: o tick **não toca** no valor. Ele muda só por canais explícitos — os `deltas` graduais de ações em execução ([§5.1](affordances.md#51-ciclo-de-vida-do-anúncio)) e eventos do jogo. Sem evento, o valor persiste indefinidamente: é o tipo certo para inventário, dinheiro e progresso de missão.
- `perception_driven`, `relationship_driven`, `context_aggregate`: leem a cada tick uma fotografia do mundo fornecida pelo chamador, em três mapas separados (percepção, relações, valores de contexto), indexados pelo `id` da consideração. Há dois modos: o jogo registra uma **função própria** que calcula o valor a partir do agente e do mundo (linha de visão, exposição, lotação), ou deixa o valor ser **lido diretamente do mapa** correspondente. O caso de borda importante: se o mundo **não trouxer** entrada para aquela consideração no tick, o valor **permanece o do tick anterior** — não zera e não decai. Um sensor que para de reportar `THREAT` congela a ameaça no último valor lido; quem quer decaimento na ausência de estímulo precisa embuti-lo na função própria.

### 4.2 Atualização por tick

```
para cada consideration c do agente:
    taxa = c.updater.rate + Σ_traits trait.consideration_delta[c.id]   // só usada pelos updaters lineares
    c.value = apply(c.updater, c, agent, world, Δt, taxa)
    c.value = clamp(c.value, c.min, c.max)                             // sempre, qualquer que seja o updater
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

Curvas úteis `[PREMISSA]` (com `n = clamp((max − v)/(max − min), 0, 1)`, a posição normalizada do valor na faixa):

| Curva | Fórmula | Uso típico |
|---|---|---|
| Convexa (urgência) | `n^e`, com `e` = `response_curve_exponent` | Necessidades críticas: o peso dispara perto do fundo. Com `e` = 2, v = -80 pesa ~4× mais que v = -20 (na faixa ±100). |
| Linear | `n` | Recursos com pressão proporcional (`AMMO`). |
| Degrau | `below` se `v < threshold`, senão `above` | Alarmes binários, estados de missão. |
| Logística | `1 / (1 + e^(−slope·(v − midpoint)))` | Transições suaves com ponto de inflexão calibrável (`FEAR`, `MORALE`). |

**Defaults e casos mudos.** Expoente 0 (ausente) numa curva convexa cai no default do perfil (`response_curve_exponent`, default 2): a curva sem expoente explícito herda a calibração global, e um expoente explícito é sempre um override deliberado por consideração. Na logística, `slope` 0 vira 1. Os patamares `below`/`above` do degrau são parâmetros livres — não precisam ser 1 e 0; um alarme pode pesar 8 abaixo do limiar e 0,5 acima. Curva com tipo desconhecido ou ausente rende pressão **0**, sem erro: a consideração fica inerte na pontuação. O mesmo ocorre com `max` ≤ `min`, por falta de amplitude para normalizar.

**Saturação:** o ganho esperado de uma ação sobre uma consideração é `min(delta, max − value)` — promessa além do teto não vale, e executar ação sobre consideração já alta rende pouco (quem acabou de descansar não valoriza a cama). O corte é **assimétrico**: ele limita apenas ganhos. Um delta negativo (a ação *custa* consideração) vale integralmente, porque `max − value` ≥ 0 nunca é menor que um delta negativo. Isso emerge da função de utilidade ([§6.2](selection.md#62-função-de-utilidade)), não de regra especial.

**Colapso:** valor no mínimo sustentado dispara consequências (desmaio, pânico, quebra de moral, fome debilitante). As consequências específicas são definidas por jogo. `[PREMISSA]`

---
