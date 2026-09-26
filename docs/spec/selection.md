# Seleção de ação e imperfeição deliberada

## 6. Algoritmo de seleção de ação

### 6.1 Visão geral

O agente pontua cada ação anunciada ao alcance combinando: (a) quanto a ação atende às considerações prementes, (b) prioridade intrínseca, (c) personalidade/traços, (d) contexto situacional, (e) distância/custo de acesso. Depois aplica seleção estocástica ([§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)). A pontuação muda o tempo inteiro, porque as considerações mudam o tempo inteiro.

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
pressure(c)         = c.base_weight * response_curve(c.value)   // [§4.3](data-model.md#43-curvas-de-resposta-valor-pressão)
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
            se x.capacity > 0 e x.occupancy >= x.capacity: continuar   // [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero): ação sem vaga
            se nao can_afford(A, x.cost): continuar
            u = score_action(A, x)                                  // [§6.2](selection.md#62-função-de-utilidade)
            candidates.adicionar((x, P, u))

    se candidates vazio: retornar default_idle_action(A)

    ordenar candidates por u decrescente
    top = primeiros SELECTION_TOP_K candidates                      // SELECTION_TOP_K = 3 [PREMISSA]

    chosen = sample_softmax(top, SELECTION_TEMPERATURE)             // [§7.1](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)
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

Quando as decisões são produzidas em lote (vários agentes de uma vez, como no `BatchDecide` da implementação de referência), dois agentes podem pontuar e escolher o mesmo recurso sem ver a decisão um do outro. A arbitragem é do serviço, não do cliente: **nenhum lote devolve mais agentes para um provedor ou para uma ação do que eles têm vagas** ([§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)).

**Critério: quem está mais perto leva.** A lógica é física, não de mérito — quem alcança o recurso antes fica com ele. Utilidade **não** é critério de desempate: "quem tem mais fome come primeiro" não existe no mundo real, fome não acelera ninguém. Registrar explicitamente, porque "maior utilidade ganha a disputa" é a escolha intuitiva que será proposta de novo — e está errada: ela faria o NPC mais necessitado teleportar prioridade para si, destruindo a legibilidade espacial da simulação.

**Mecânica.** A pontuação continua paralela (cada agente é pontuado independentemente); só a reconciliação é serial:

1. Cada agente produz sua **lista ordenada de preferência**: a primeira posição é a escolha **estocástica** usual (softmax sobre os `SELECTION_TOP_K` melhores, [§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)) — a imperfeição deliberada sobrevive à arbitragem, a arbitragem não vira argmax. As posições seguintes são os demais candidatos em ordem decrescente de utilidade, servindo apenas de plano B. A lista é limitada a `RECONCILIATION_TOP_K` candidatos (default 5): em lotes de dezenas de milhares de agentes, reter a lista completa por agente estoura memória.
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
| 2 | **Compromisso com ação subótima** | Uma vez iniciada, a ação só é interrompida por urgência alta ([§8.3](execution.md#83-interrupção-preempção)), mesmo se outra opção a superar | `preemption_margin` (default 1,5×) `[PREMISSA]` | Margem↑ → agente "teimoso", cenas cômicas/dramáticas; margem↓ → agente reativo/robótico. |
| 3 | **Sem otimização global** | O agente só vê anúncios ao alcance; não planeja rotas nem agenda de longo prazo | `advertisement_radius` por provedor | Raio↑ → decisões mais "espertas", menos deambulação. |
| 4 | **Convenções imperfeitas** | Não implementar (ou probabilisticamente ignorar) regras sociais/táticas "corretas" | `convention_break_probability` (default 0,15) `[PREMISSA]` | ↑ → mais situações estranhas/engraçadas/memoráveis. |
| 5 | **Traços que invertem incentivos** | Traços como `inappropriate` ou `reckless` bonificam o comportamento "errado" | multiplicadores do traço | ↑ → mais gafes, riscos absurdos, drama. |
| 6 | **Percepção ruidosa** `[PREMISSA]` | Considerações `perception_driven` recebem ruído ou atraso de atualização | `perception_noise` (default σ = 5 pts) | ↑ → erros de leitura do mundo (NPC não percebe a ameaça óbvia). |

**Diretriz de tuning:** o alvo não é minimizar erro, é maximizar **variedade legível**: o observador deve conseguir contar uma história sobre por que o agente fez aquilo.

---
