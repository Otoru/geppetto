# Seleção de ação e imperfeição deliberada

## 6. Algoritmo de seleção de ação

### 6.1 Visão geral

A seleção tem três etapas, nesta ordem:

1. **Elegibilidade:** filtrar os anúncios que o agente pode executar agora (alcance do anúncio, precondições, vagas nos dois níveis, custo pagável, compromissos narrativos).
2. **Pontuação:** atribuir uma utilidade a cada anúncio elegível ([§6.2](selection.md#62-função-de-utilidade)).
3. **Escolha estocástica:** amostrar entre os melhores pontuados, em vez de tomar o máximo ([§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)).

A pontuação combina: (a) quanto a ação atende às considerações prementes, (b) prioridade intrínseca, (c) personalidade/traços, (d) contexto situacional, (e) distância/custo de acesso. Ela muda o tempo inteiro, porque as considerações mudam o tempo inteiro — a ação que liderava há uma hora de jogo pode nem entrar entre os finalistas agora.

### 6.2 Função de utilidade

Para agente `A` e anúncio `x` (ação oferecida pelo provedor `P`):

```
score_action(A, x) = ( B(A, x) + x.intrinsic_priority * W_PRIORITY )
                     * M_personality(A, x)
                     * M_context(A, x)
                     * M_distance(A, P)
                     - weighted_cost(A, x)

B(A, x) = Σ_c  pressure(c) * expected_gain(c, x)      // só considerações que A possui
```

Componentes:

```
expected_gain(c, x) = min( x.deltas[c], c.max - c.value )       // saturação: promessa além do teto não vale
pressure(c)         = c.base_weight * response_curve(c.value)   // [§4.3](data-model.md#43-curvas-de-resposta-valor-pressão)
M_personality       = Π_t∈A.personality.traits  Π_tag∈tags(x)  t.modifiers[tag]  *  (1 + preference(A, domain(x)) / 10)
M_context           = Π_k∈A.active_contexts  m_k(x) * m_k(x),   onde m_k(x) = Π_tag∈tags(x)  k.modifiers[tag]
M_distance          = 1 / (1 + distance(A, P) / DISTANCE_REFERENCE)   // DISTANCE_REFERENCE ≈ 10 m de jogo [PREMISSA]
weighted_cost       = Σ_r  x.cost[r] / max(1, available(A, r) / x.cost[r])
```

(Nos produtórios sobre tags, só participam as tags presentes no mapa de modificadores do traço ou do contexto; tag sem modificador contribui ×1.)

A ordem dos operadores é normativa: a prioridade intrínseca **soma-se ao benefício**, o total é escalado pelos três multiplicadores, e o custo é subtraído **por último, sem escala**.

**Por que cada fator existe:**

- **Pressão da consideração (`pressure`).** O valor bruto da consideração não diz quão urgente ela é; a curva de resposta diz. Curvas convexas (expoente default 2, ajustável por perfil e sobrescrito por consideração) fazem a pressão disparar perto do fundo: sob fogo pesado, `THREAT` esmaga `FUN` sem nenhuma regra especial de "modo de combate". É por aqui que a urgência entra no score.
- **Ganho esperado com saturação (`expected_gain`).** Uma promessa só vale até o teto da consideração: quem acabou de descansar não valoriza a cama. Sem o clamp, ações de delta grande dominariam mesmo quando o agente não tem o que ganhar. Deltas negativos entram na mesma soma (promessas mistas, ex.: +`FUN`/−`ENERGY`), e deltas sobre considerações que o agente não possui são ignorados.
- **Prioridade intrínseca (`W_PRIORITY`, default 5).** Permite ao autor de conteúdo marcar uma ação como importante independentemente do estado interno do agente — o quest giver que precisa ser visitado, o alarme que precisa ser tocado. Por ser somada ao benefício **antes** dos multiplicadores, uma prioridade longe continua penalizada pela distância: prioridade não teletransporta.
- **Personalidade (`M_personality`).** Multiplicativa sobre as tags da ação, mais um fator de domínio: o NPC erudito tem `preference(book) > 0` (preference +8 ⇒ ×1,8); o `sadistic` tem multiplicador > 1 em tags {`intimidate`, `torture`}. Multiplicar (e não somar) preserva a proporcionalidade: um traço amplia ou reduz o benefício inteiro, em vez de criar atração por ações que não atendem a nada.
- **Contexto (`M_context`) — aplicado duas vezes.** Cada contexto ativo contribui seu multiplicador de tag **ao quadrado**: um `location:gym` ×2,0 em `exercise` entra como ×4,0 sobre o benefício. O reforço é deliberado: aplicado uma única vez, o efeito do contexto se dilui na seleção estocástica e a frequência observável de ações da tag não chega a dobrar (CA-9). Não "corrigir" para aplicação simples sem recalibrar o critério de aceitação.
- **Distância (`M_distance`).** Decaimento hiperbólico `[PREMISSA]`: a `DISTANCE_REFERENCE` metros o benefício vale metade; ao dobro, um terço. Acessibilidade física é parte do valor de uma ação — o melhor restaurante da cidade não compete com o boteco da esquina para quem está faminto agora. Pode ser substituída por custo de pathfinding real quando disponível.
- **Custo (`weighted_cost`).** Para uma ação pagável (a elegibilidade já garantiu `available ≥ cost`), a penalidade por recurso reduz a `cost² / available`: **superlinear no custo e inversamente proporcional ao estoque** — dobrar o custo quadruplica a penalidade; ver o estoque cair pela metade dobra a penalidade do mesmo custo. A escassez pesa sem precisar de uma tabela de escassez separada. E por ser subtraído depois dos multiplicadores, o custo nunca é "descontado" por um modificador positivo (personalidade não paga a conta) nem abrandado pela distância.

### 6.3 Pseudocódigo (neutro de linguagem)

```
funcao select_action(agent A) -> ActionInstance ou nulo:
    candidates = []
    para cada provider P:
        se P.capacity - |P.occupants| <= 0: continuar               // provedor sem vaga [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)
        para cada action x em P.advertised_actions:
            se distance(A, P) > x.advertisement_radius: continuar   // fora do alcance do anúncio
            se x.capacity > 0 e x.occupancy >= x.capacity: continuar   // ação sem vaga [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)
            se nao preconditions_hold(A, P.state, x.preconditions): continuar   // capability, recurso mínimo, estado do provedor
            se nao can_afford(A, x.cost): continuar
            se algum NarrativeCommitment de A contradiz tags(x): continuar      // [§8.2](execution.md#82-fila-de-ações-actionqueue)
            u = score_action(A, x)                                  // [§6.2](selection.md#62-função-de-utilidade)
            candidates.adicionar((x, P, u))

    se candidates vazio: retornar nulo
        // Não existe "ação idle default" no motor: sem candidato elegível, a
        // resposta é "sem ação", e cabe ao jogo decidir o que o NPC faz parado.

    ordenar candidates por u decrescente
        // ordenação estável: empate de utilidade mantém a ordem de descoberta
    top = primeiros min(SELECTION_TOP_K, |candidates|) candidates   // SELECTION_TOP_K = 3 [PREMISSA]

    chosen = sample_softmax(top, SELECTION_TEMPERATURE)             // [§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)
    retornar instantiate(chosen, A)
        // a instância criada registra u como continuation_utility — o retrato
        // congelado contra o qual a preempção futura compara [§8.3](execution.md#83-interrupção-preempção)

funcao sample_softmax(candidates, T):
    u_max = maior u entre candidates
    w_i = e^((u_i - u_max) / T)        // subtrair o máximo não muda a distribuição;
                                       // evita overflow numérico com utilidades grandes
    sortear i com probabilidade w_i / Σ_j w_j
```

**Valor de tuning ausente cai no default.** Em todos os parâmetros numéricos do motor, um valor zero (ou negativo, onde não faz sentido) no tuning é substituído pelo default desta especificação — vale para `SELECTION_TOP_K`, `SELECTION_TEMPERATURE`, `W_PRIORITY`, `DISTANCE_REFERENCE`, `RECONCILIATION_TOP_K`, `preemption_margin` e `response_curve_exponent`. Consequência: temperatura 0 não é configurável; o argmax é o limite `T → 0⁺`, nunca um valor que se possa setar por engano.

### 6.4 Contextos situacionais

Ao entrar num local, assumir um papel ou mudar de estado de missão, o agente recebe um `Context { modifiers: mapa(tag → real), duration }`. Contextos são o mecanismo pelo qual o ambiente "sugere" comportamento sem codificar regras no agente. A duração é gerida pelo jogo: expirado, o contexto sai da lista de ativos. Exemplos `[PREMISSA]`:

| Contexto | Efeito |
|---|---|
| `location:gym` | ×2,0 em tag `exercise` |
| `role:host` | ×1,5 em `social`, ×1,3 em `serve` |
| `role:guest` | ×1,3 em `social`, ×0,7 em `invasive` |
| `state:under_fire` | ×3,0 em `defensive`, ×0,3 em `social` |
| `mission:escort` | ×2,5 em `protect-target` |
| `trait:inappropriate` | ×2,0 em `break-convention` (inversão deliberada de incentivos sociais) |

**Os multiplicadores da tabela são nominais.** Na pontuação, cada contexto entra ao quadrado ([§6.2](selection.md#62-função-de-utilidade)): `location:gym` contribui ×4,0 sobre o benefício de ações `exercise`, não ×2,0. A tabela expressa a intenção de design ("este lugar pesa o dobro"); o motor amplifica para que a intenção sobreviva à seleção estocástica.

### 6.5 Arbitragem de disputa dentro do lote

Quando as decisões são produzidas em lote (vários agentes de uma vez, como na chamada `BatchDecide` do contrato de wire), dois agentes podem pontuar e escolher o mesmo recurso sem ver a decisão um do outro. A arbitragem é do serviço, não do cliente: **nenhum lote devolve mais agentes para um provedor ou para uma ação do que eles têm vagas** ([§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)).

**Critério: quem está mais perto leva.** A lógica é física, não de mérito — quem alcança o recurso antes fica com ele. Utilidade **não** é critério de desempate: "quem tem mais fome come primeiro" não existe no mundo real, fome não acelera ninguém. Registrar explicitamente, porque "maior utilidade ganha a disputa" é a escolha intuitiva que será proposta de novo — e está errada: ela faria o NPC mais necessitado teleportar prioridade para si, destruindo a legibilidade espacial da simulação.

**Mecânica.** A pontuação continua paralela (cada agente é pontuado independentemente); só a reconciliação é serial:

1. Cada agente produz sua **lista ordenada de preferência**: a primeira posição é a escolha **estocástica** usual (softmax sobre os `SELECTION_TOP_K` melhores, [§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)) — a imperfeição deliberada sobrevive à arbitragem, a arbitragem não vira argmax (CA-21). As posições seguintes são os demais candidatos em ordem decrescente de utilidade, servindo apenas de plano B — inclusive candidatos que ficaram fora do top-K da seleção. A lista é limitada a `RECONCILIATION_TOP_K` candidatos **no total** (default 5; zero no tuning cai no default): em lotes de dezenas de milhares de agentes, reter a lista completa por agente estoura memória.
2. A reconciliação distribui as vagas em **rodadas**. Em cada rodada, todo agente ainda em aberto reivindica o candidato apontado pelo seu cursor na própria lista; as reivindicações são agrupadas por provedor; dentro de cada provedor, os contendores são ordenados por **distância crescente àquele provedor**, com desempate explícito por ID do agente. As vagas são concedidas nessa ordem, respeitando os dois níveis de capacidade: vagas livres do provedor (`capacity − |occupants|`) e, por ação, `capacity − occupancy` (0 = sem limite). O limite por ação é **por instância de provedor**: a serra da bancada X não divide sua vaga com a serra da bancada Y. Quem não obtém vaga avança o cursor para o próximo candidato da **própria** lista — o que pode criar disputa nova em outro provedor.
3. **Atribuições são provisórias até estabilizar.** Um agente que conseguiu vaga numa rodada pode perdê-la na seguinte, se um contendente mais próximo, desalojado de outro provedor, cair em cascata sobre o mesmo recurso. O processo repete até que uma rodada inteira passe sem nenhum desalojamento.
4. Agente que esgotar os candidatos sai sem ação (`selected_action_index = -1` no contrato de wire), como no caso de nenhum anúncio elegível.

**Exemplo de cascata.** Cadeira e sofá no mesmo ponto, uma vaga cada; o agente mais próximo (0,5 m) só quer a cadeira; o do meio (1 m) quer a cadeira e depois o sofá; o mais distante (2 m) só quer o sofá. Rodada 1: o mais próximo fica com a cadeira; o do meio perde e avança para o sofá; o mais distante recebe o sofá **provisoriamente**. Rodada 2: o do meio e o mais distante disputam o sofá; o do meio está mais perto e toma a vaga; o mais distante não tem plano B e sai sem ação (CA-18).

**Terminação garantida:** o cursor de um agente na própria lista só anda para frente — desalojado do candidato `i`, ele nunca mais aponta para `i`. Cada agente é desalojado de cada candidato no máximo uma vez, logo o total de desalojamentos é limitado pela soma dos tamanhos das listas (finita, porque cada lista tem no máximo `RECONCILIATION_TOP_K` entradas). Não há laço infinito.

**Determinismo inegociável:** a ordem da reconciliação é explícita e estável — distância crescente, desempate por ID do agente. Nunca depende de ordem de iteração de mapa nem de ordem de conclusão de goroutine: cada agente aparece em exatamente uma lista de reivindicação por rodada, portanto a ordem em que os provedores são processados não influencia o resultado. Mesma entrada + mesma seed = mesma atribuição, inclusive nas disputas.

**Statelessness preservada:** a arbitragem acontece inteiramente dentro de uma chamada; o serviço não guarda mundo entre requisições. A reconciliação só precisa de identidade, posição e lista de preferências de cada agente — as utilidades já cumpriram seu papel ordenando as listas.

---

## 7. Fontes deliberadas de imperfeição e como sintonizá-las

O agente não deve escolher sempre a melhor opção. O erro faz parte da ilusão de vida e protege o jogo contra exploração pelo jogador.

| # | Fonte de imperfeição | Mecanismo | Parâmetro de sintonia | Efeito de aumentar |
|---|---|---|---|---|
| 1 | **Seleção estocástica** | Softmax sobre os `SELECTION_TOP_K` melhores candidatos: `P(i) = e^(u_i / SELECTION_TEMPERATURE) / Σ_j e^(u_j / SELECTION_TEMPERATURE)`, computado com subtração do máximo (mesma distribuição, sem overflow) | `SELECTION_TEMPERATURE` (default 1,0) e `SELECTION_TOP_K` (default 3) `[PREMISSA]` | `SELECTION_TEMPERATURE`↑ → escolhas mais aleatórias; `T → 0⁺` → argmax (limite inatingível por tuning: valor ≤ 0 cai no default, [§6.3](selection.md#63-pseudocódigo-neutro-de-linguagem)). `SELECTION_TOP_K`↑ → mais variedade; `SELECTION_TOP_K` = 1 degenera o softmax em argmax. |
| 2 | **Compromisso com ação subótima** | Uma vez iniciada, a ação só é interrompida por urgência alta ([§8.3](execution.md#83-interrupção-preempção)), mesmo se outra opção a superar; a comparação usa a utilidade congelada no instante da escolha | `preemption_margin` (default 1,5×) `[PREMISSA]` | Margem↑ → agente "teimoso", cenas cômicas/dramáticas; margem↓ → agente reativo/robótico. |
| 3 | **Sem otimização global** | O agente só vê anúncios ao alcance; não planeja rotas nem agenda de longo prazo | `advertisement_radius` por provedor | Raio↑ → decisões mais "espertas", menos deambulação. |
| 4 | **Convenções imperfeitas** | Não implementar (ou probabilisticamente ignorar) regras sociais/táticas "corretas" | `convention_break_probability` (default 0,15) `[PREMISSA]` | ↑ → mais situações estranhas/engraçadas/memoráveis. |
| 5 | **Traços que invertem incentivos** | Traços como `inappropriate` ou `reckless` bonificam o comportamento "errado" | multiplicadores do traço | ↑ → mais gafes, riscos absurdos, drama. |
| 6 | **Percepção ruidosa** `[PREMISSA]` | Considerações `perception_driven` recebem ruído ou atraso de atualização | `perception_noise` (default σ = 5 pts) | ↑ → erros de leitura do mundo (NPC não percebe a ameaça óbvia). |

A imperfeição atravessa o sistema inteiro: a escolha estocástica também encabeça a lista de preferências na arbitragem de lote ([§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote)) — a reconciliação não a remove — e a preempção usa a mesma seleção estocástica ([§8.3](execution.md#83-interrupção-preempção)), de modo que mesmo sob urgência o agente ocasionalmente troca para o segundo melhor alívio.

**Diretriz de tuning:** o alvo não é minimizar erro, é maximizar **variedade legível**: o observador deve conseguir contar uma história sobre por que o agente fez aquilo.

---
