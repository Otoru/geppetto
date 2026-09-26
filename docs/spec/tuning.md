# Parâmetros de tuning consolidados

## 10. Parâmetros de tuning consolidados

Todos os valores default são `[PREMISSA]`.

| Parâmetro | Default | Faixa | Seção |
|---|---|---|---|
| Faixa de consideração (`value`) | [-100, +100] (ou [0, 100] unipolar) | fixo | [3.1](data-model.md#31-consideration) |
| `response_curve_exponent` (convexidade da urgência) | 2,0 | [1, 4] | [4.3](data-model.md#43-curvas-de-resposta-valor-pressão) |
| `decay_rate` por consideração | ver [§4.2](data-model.md#42-atualização-por-tick) | 3–20 pts/h | [4.2](data-model.md#42-atualização-por-tick) |
| `critical_threshold` por consideração | ver [§4.2](data-model.md#42-atualização-por-tick) | [-90, -10] | [4.2](data-model.md#42-atualização-por-tick) |
| `W_PRIORITY` (peso da prioridade intrínseca) | 5 | [0, 20] | [6.2](selection.md#62-função-de-utilidade) |
| `DISTANCE_REFERENCE` (distância de referência) | 10 m | [2, 50] | [6.2](selection.md#62-função-de-utilidade) |
| `SELECTION_TOP_K` (candidatos na seleção) | 3 | [1, 10] | [6.3](selection.md#63-pseudocódigo-neutro-de-linguagem) |
| `SELECTION_TEMPERATURE` (temperatura softmax) | 1,0 (0,3–1,5 por gênero, [§9](genres.md#9-instanciação-por-gênero)) | [0,1, 5] | [7.1](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) |
| `preemption_margin` | 1,5× | [1,0, 3,0] | [7.2](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) / [8.3](execution.md#83-interrupção-preempção) |
| `convention_break_probability` | 0,15 | [0, 1] | [7.4](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) |
| `perception_noise` (σ) | 5 pts | [0, 30] | [7.6](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) |
| Multiplicadores de traço (`Trait.modifiers`) | 0,5–2,0 | [0, 5] | [3.2](data-model.md#32-trait-e-personality) / [6.4](selection.md#64-contextos-situacionais) |
| Tick `FULL` / `SIMPLIFIED` | 1 min / 1 h de jogo | — | [8.4](execution.md#84-níveis-de-detalhe-de-simulação-lod) |
| `capacity` default de provedor | 1 | [0, ∞) | [3.4](data-model.md#34-affordanceprovider) |
| `capacity` de ação | 0 (**sem limite** — [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)) | [0, ∞) | [3.5](data-model.md#35-advertisedaction) / [5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero) |
| `RECONCILIATION_TOP_K` (candidatos retidos por agente) | 5 | [1, ∞) | [6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote) |
| `advertisement_radius` default | mesmo cômodo/zona (~8 m) | [1, 100] m | [3.5](data-model.md#35-advertisedaction) |

---
