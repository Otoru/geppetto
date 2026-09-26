# Ciclo de execução, fila, preempção e arbitragem de disputa

## 8. Ciclo de execução

### 8.1 Loop de atualização (por tick, por agente em nível `FULL`)

```
1. update_considerations(A, Δt)                        // [§4](data-model.md#4-atualizadores-de-consideração) (todos os updaters)
2. se A.current_action != nulo:
       apply_gradual_deltas(A.current_action, Δt)
       se is_action_complete(A.current_action): A.current_action = nulo
3. se A.current_action == nulo e action_queue vazia:
       a = select_action(A)                             // [§6.3](selection.md#63-pseudocódigo-neutro-de-linguagem)
       se a: start_action(a)
4. senao: check_preemption(A)                           // [§8.3](execution.md#83-interrupção-preempção)
5. process_queue(A)                                     // [§8.2](execution.md#82-fila-de-ações-actionqueue)
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

- **`FULL`:** loop de [§8.1](execution.md#81-loop-de-atualização-por-tick-por-agente-em-nível-full), tick fino (ex.: 1 s–1 min de jogo). `[PREMISSA]`
- **`SIMPLIFIED`:** tick grosso (ex.: 1 h de jogo); considerações avançam por modelo agregado; decisões são eventos discretos. `[PREMISSA]`
- **Transição `SIMPLIFIED` → `FULL`:** inicializar considerações com valores condizentes com a agenda simulada e a hora do dia. `[PREMISSA]` no mecanismo.

---
