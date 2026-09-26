# Instanciação por gênero

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
