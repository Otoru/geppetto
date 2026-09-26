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
| `considerations` | mapa (id → `Consideration`) | [§3.1](data-model.md#31-consideration) |
| `personality` | `Personality` | [§3.2](data-model.md#32-trait-e-personality) |
| `action_queue` | `ActionQueue` (lista ordenada de `ActionInstance`) | [§8](execution.md#8-ciclo-de-execução); ações em andamento e enfileiradas. |
| `current_action` | `ActionInstance` ou nulo | |
| `position` | coordenada no mundo | Usada no termo de distância ([§6](selection.md#6-algoritmo-de-seleção-de-ação)). |
| `capabilities` | conjunto (skills, facção, classe, equipamento) | Alimenta precondições de anúncios ([§5.2](affordances.md#52-anúncios-dinâmicos-e-condicionais)). |
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

### 3.5 `AdvertisedAction`

| Campo | Tipo | Faixa / unidade | Notas |
|---|---|---|---|
| `action_id` | string | ex.: `eat`, `reload`, `flank`, `converse`, `harvest` | |
| `deltas` | mapa (consideração → real) | [-100, +100] por consideração | Quanto a ação **promete** alterar cada consideração por execução completa. |
| `estimated_duration` | real | segundos/minutos de jogo, > 0 | `[PREMISSA]` |
| `tags` | conjunto de strings | ex.: {`social`, `combat`, `defensive`, `work`, `mischief`} | Alvo dos modificadores de traço/contexto. `[PREMISSA]` |
| `domain` | string | ex.: `book`, `arena` | Domínio de preferência consultado em `Personality.preferences` via `preference(A, domain(x))` ([§6.2](selection.md#62-função-de-utilidade)). `[PREMISSA]` |
| `preconditions` | lista de predicados sobre o agente | | Skill, facção, estado, capacidade livre, posse de item. Ver [§5.2](affordances.md#52-anúncios-dinâmicos-e-condicionais). |
| `intrinsic_priority` | real | [0, 10] | Algumas ações têm prioridade estrutural (ex.: ordens diretas, reações de sobrevivência). |
| `advertisement_radius` | real | metros de jogo | Alcance em que o anúncio é visível. `[PREMISSA]` |
| `cost` | mapa (recurso → quantidade) | ≥ 0 | Ações podem consumir recursos (munição, stamina, dinheiro). `[PREMISSA]` |
| `capacity` | inteiro ≥ 0 | Quantos agentes podem executar **esta ação** simultaneamente. **0 = sem limite** — deliberadamente diferente de `AffordanceProvider.capacity`, onde 0 bloqueia. O motivo e as consequências estão em [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero). |
| `occupancy` | inteiro ≥ 0 | Quantos agentes já executam esta ação no início do tick, informado pelo cliente; consome vagas da ação antes de qualquer decisão nova ([§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)). |

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

**Saturação:** o ganho de uma ação é limitado pelo teto da consideração — executar ação sobre consideração já alta rende pouco (quem acabou de descansar não valoriza a cama). Isso emerge da função de utilidade ([§6.2](selection.md#62-função-de-utilidade)), não de regra especial.

**Colapso:** valor no mínimo sustentado dispara consequências (desmaio, pânico, quebra de moral, fome debilitante). As consequências específicas são definidas por jogo. `[PREMISSA]`

---
