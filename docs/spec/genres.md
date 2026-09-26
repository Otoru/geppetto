# Instanciação por gênero

## 9. Instanciação por gênero

A mesma maquinaria (`considerations → response curves → utility → stochastic selection`) configurada para quatro casos distintos. Nada muda no motor; mudam as considerações, os provedores, as tags e o tuning.

Os quatro gêneros abaixo correspondem aos quatro **perfis reais distribuídos com o motor** (`social-life`, `tactical-stealth`, `survival-crafting`, `open-world-rpg`). Cada perfil é deliberadamente **mínimo**: três a quatro considerações bastam para demonstrar o loop de decisão do gênero. As tabelas de considerações e o tuning reproduzem fielmente o que o perfil carrega; provedores, tags e traços **não fazem parte do arquivo de perfil** — são orientação de como o cliente deve povoar o mundo para aquele gênero. Ao final de cada seção, "extensões sugeridas" lista o que uma produção real adicionaria — e que o perfil mínimo **não** contém.

Nenhum dos quatro perfis configura `RECONCILIATION_TOP_K`: todos usam o default 5 ([§10](tuning.md#10-parâmetros-de-tuning-consolidados)).

### 9.1 Simulação social / vida (`social-life`)

| Consideração | Atualizador | Taxa | `base_weight` | `critical_threshold` | Curva |
|---|---|---|---|---|---|
| `HUNGER` | `linear_decay` | 10 pts/h | 3 | -50 | convexa (exp. 2) |
| `ENERGY` | `linear_decay` | 12,5 pts/h | 3 | -60 | convexa (exp. 2) |
| `FUN` | **nenhum** | — | 1 | -30 | convexa (exp. 2) |

| Tuning | Valor |
|---|---|
| `SELECTION_TEMPERATURE` | 1,2 |
| `preemption_margin` | 1,7 |
| `W_PRIORITY` | 2 |
| `SELECTION_TOP_K` / `DISTANCE_REFERENCE` / `response_curve_exponent` | 3 / 10 / 2 (defaults) |
| `convention_break_probability` / `perception_noise` | 0,15 / 5 (defaults) |

**Por que assim.** O gênero vende **história emergente**, não competência: temperatura 1,2 (a mais alta dos quatro perfis) achata as diferenças de utilidade e faz o NPC escolher a opção "errada" com frequência suficiente para gerar enredo — ele vai à festa com fome porque quis. A margem de preempção 1,7 (a mais alta) torna o NPC teimoso: ele termina a conversa antes de atender a bexiga, e a cena cômica nasce desse atraso. `W_PRIORITY` 2 (o mais baixo) tira o peso de prioridades impostas de fora: em simulação social, personalidade e necessidade devem mandar mais que roteiro. `FUN` com `base_weight` 1 e **sem atualizador** é uma escolha deliberada do perfil mínimo: o tédio não decai sozinho — só muda quando o NPC age ou o jogo envia um evento — e, com peso 1, diversão é luxo que nunca atropela fome (peso 3).

**Orientação de mundo (cliente).** Provedores: mobília e eletrodomésticos, locais de lazer, outros NPCs (socialização), eventos (festas). Tags típicas: `social`, `leisure`, `domestic`, `mischief`, `romance`. Traços exemplares: `lazy` (×1,5 em `sit`), `erudite` (preferência por livros), `mean` (×2 em `provoke`), `inappropriate` (×2 em `break-convention`).

**Extensões sugeridas (não estão no perfil):** `HYGIENE`, `BLADDER`, `COMFORT`, `SOCIAL`, `ENVIRONMENT` — todas `linear_decay` com curva convexa, pesos 1–2, seguindo o mesmo padrão de `HUNGER`/`ENERGY`.

### 9.2 Combate tático / stealth (`tactical-stealth`)

| Consideração | Atualizador | Taxa | `base_weight` | `critical_threshold` | Curva |
|---|---|---|---|---|---|
| `THREAT` | `perception_driven` | — | 5 | -40 | convexa (exp. 2) |
| `COVER` | `perception_driven` | — | 4 | -30 | linear |
| `AMMO` | `event_driven` | — | 3 | **+10** | linear |

| Tuning | Valor |
|---|---|
| `SELECTION_TEMPERATURE` | 0,5 |
| `preemption_margin` | 1,2 |
| `W_PRIORITY` | 5 |
| `SELECTION_TOP_K` / `DISTANCE_REFERENCE` / `response_curve_exponent` | 3 / 10 / 2 (defaults) |
| `convention_break_probability` / `perception_noise` | 0,15 / 5 (defaults) |

**Por que assim.** Competência tática **vende o gênero**: temperatura 0,5 (a mais baixa) faz o melhor candidato ganhar quase sempre — o soldado flanqueia, usa cobertura, recarrega na janela certa. A imperfeição não some; ela **migra da seleção para a percepção**: `perception_noise` > 0 produz erros de detecção críveis (o guarda que não viu o intruso) sem comprometer a qualidade da decisão. A margem de preempção 1,2 (a mais baixa) torna o NPC ultra-reativo: `THREAT` cruzando -40 interrompe qualquer ação quase imediatamente — sobreviver vem antes de terminar o que quer que ele estivesse fazendo. `THREAT` com peso 5 e curva convexa esmaga qualquer outra pressão quando dispara: sob fogo, nada mais existe. `AMMO` é o caso didático de consideração **unipolar** ([0, 100]) com limiar **positivo**: "10 balas" não é conforto, é emergência — por isso `critical_threshold` +10. `COVER` linear porque o valor de uma cobertura deve ser proporcional à exposição, sem explosão de urgência.

**Orientação de mundo (cliente).** Provedores: zonas de cobertura, pontos de flanco, posições elevadas, corpos saqueáveis, alarmes, rotas de patrulha, aliados (anunciam `heal`, `cover`). Tags típicas: `offensive`, `defensive`, `stealth`, `support`, `retreat`. Traços exemplares: `cautious` (×2 em `defensive`), `reckless` (×2 em `offensive`, inversão deliberada), `loyal` (×2 em `protect-ally`).

**Extensões sugeridas (não estão no perfil):** `FATIGUE` (`linear_decay` em combate), `VISIBILITY` (`perception_driven` — quanto o inimigo me vê), `MORALE` (`context_aggregate` de grupo). `THREAT` com curva logística é uma alternativa legítima para transição suave de alerta; o perfil distribuído usa convexa.

### 9.3 Survival / crafting (`survival-crafting`)

| Consideração | Atualizador | Taxa | `base_weight` | `critical_threshold` | Curva |
|---|---|---|---|---|---|
| `HUNGER` | `linear_decay` | 10 pts/h | 4 | -50 | convexa (exp. 2) |
| `THIRST` | `linear_decay` | 12 pts/h | 4 | -55 | convexa (exp. 2) |
| `ENERGY` | `linear_decay` | 12,5 pts/h | 3 | -60 | convexa (exp. 2) |

| Tuning | Valor |
|---|---|
| `SELECTION_TEMPERATURE` | 0,8 |
| `preemption_margin` | 1,5 |
| `W_PRIORITY` | 5 |
| `SELECTION_TOP_K` / `DISTANCE_REFERENCE` / `response_curve_exponent` | 3 / 10 / 2 (defaults) |
| `convention_break_probability` / `perception_noise` | 0,15 / 5 (defaults) |

**Por que assim.** O gênero é um **loop de sobrevivência**: os limiares críticos agressivos (-50, -55, -60, os mais altos dos quatro perfis em valor absoluto) e os pesos 4 em `HUNGER`/`THIRST` garantem que necessidades fisiológicas dominem a agenda cedo e com frequência — o jogador deve *ver* o NPC preso no ciclo comer-beber-dormir. `THIRST` decai mais rápido que `HUNGER` (12 vs 10 pts/h) e tem limiar mais alto: sede aperta antes da fome, como na biologia real. Temperatura 0,8 (intermediária) mantém o NPC competente no essencial sem virar robô: survival tolera erro ocasional (colheu a planta errada), mas não no limiar da morte. Custos de recursos devem ter peso alto na configuração do cliente — escassez precisa doer para o loop de crafting fechar.

**Orientação de mundo (cliente).** Provedores: recursos coletáveis (planta, minério, caça), estações de crafting, fogueira, abrigo, fontes de água, clima (evento que anuncia `seek_shelter`). `advertisement_radius` grande para recursos (exploração) e pequeno para estações. Tags típicas: `gathering`, `construction`, `cooking`, `hunting`, `rest`. Traços exemplares: `hardworking` (×1,5 em `gathering`), `gluttonous` (×1,5 em `cooking`), `nomadic` (×0,7 em `construction`).

**Extensões sugeridas (não estão no perfil):** `COLD`/`HEAT` (`perception_driven` ambiental), `INJURY` (`event_driven`), `STOCK(resource)` (`event_driven`), `SHELTER_SAFETY` (`context_aggregate`).

### 9.4 NPC de vila / ambiente em RPG de mundo aberto (`open-world-rpg`)

| Consideração | Atualizador | Taxa | `base_weight` | `critical_threshold` | Curva |
|---|---|---|---|---|---|
| `ENERGY` | `linear_decay` | 12,5 pts/h | 3 | -60 | convexa (exp. 2) |
| `HUNGER` | `linear_decay` | 10 pts/h | 3 | -50 | convexa (exp. 2) |
| `DUTY` | `linear_decay` | 8 pts/h | 2 | -40 | convexa (exp. 2) |
| `SECURITY` | `perception_driven` | — | 5 | -40 | convexa (exp. 2) |

| Tuning | Valor |
|---|---|
| `SELECTION_TEMPERATURE` | 1,2 |
| `preemption_margin` | 1,7 |
| `W_PRIORITY` | 2 |
| `SELECTION_TOP_K` / `DISTANCE_REFERENCE` / `response_curve_exponent` | 3 / 10 / 2 (defaults) |
| `convention_break_probability` / `perception_noise` | 0,15 / 5 (defaults) |

**Por que assim.** Vilas precisam parecer **vivas e imprevisíveis**: temperatura 1,2 e margem 1,7 (idênticas às da simulação social) produzem rotinas com desvios legíveis — o ferreiro fecha a loja mais cedo para beber na taverna. A rotina de trabalho é modelada como `DUTY` com decaimento lento (8 pts/h, o mais lento dos quatro perfis) e peso baixo (2): o dever é uma pressão **suave e contínua**, não uma urgência — ele puxa o NPC de volta ao posto ao longo do dia, mas perde para fome real e para qualquer emergência. `SECURITY` é o oposto: `perception_driven`, peso 5 (o maior do perfil), limiar -40 — quando perigo entra em cena, ele atropela rotina, fome e dever, e a vila inteira reage (fuga, alarme, defesa). `W_PRIORITY` 2 mantém o protagonismo na vida interna do NPC, não em roteiros externos. LOD agressivo é parte do tuning do gênero: a maioria dos NPCs roda em `SIMPLIFIED` ([§8.4](execution.md#84-níveis-de-detalhe-de-simulação-lod)), e `narrative_commitments` fortes impedem que NPCs de missão abandonem o posto por `HUNGER` não-crítica.

**Orientação de mundo (cliente).** Provedores: lojas e postos de trabalho (anunciam `work`), taverna, templo, casa, praça, o próprio jogador (anuncia `converse`, `trade`), eventos dinâmicos (incêndio, festival, ataque). Tags típicas: `work`, `social`, `devotion`, `commerce`, `flee`. Traços exemplares: `devout` (×2 em `devotion`), `greedy` (×1,5 em `commerce`), `cowardly` (×3 em `flee` quando `SECURITY` crítica).

**Extensões sugeridas (não estão no perfil):** `BOND(player)` (`relationship_driven`), `VILLAGE_PROSPERITY` (`context_aggregate`), `CURIOSITY` (`linear_decay` lento — atrai para eventos).

---
