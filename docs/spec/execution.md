# Ciclo de execução, fila, preempção e arbitragem de disputa

## 8. Ciclo de execução

### 8.1 Loop de atualização (por tick, por agente em nível `FULL`)

```
1. update_considerations(A, Δt)                        // todos os updaters [§4](data-model.md#4-atualizadores-de-consideração)
2. se A.current_action != nulo:
       apply_gradual_deltas(A.current_action, Δt)      // taxa constante: delta * Δt / estimated_duration
       se elapsed >= estimated_duration: A.current_action = nulo
3. se A.current_action == nulo e action_queue vazia:
       a = select_action(A)                             // [§6.3](selection.md#63-pseudocódigo-neutro-de-linguagem)
       se a: start_action(a)
4. senao: check_preemption(A)                           // [§8.3](execution.md#83-interrupção-preempção)
```

A ordem dos passos é normativa, e cada detalhe tem efeito observável:

- **Considerações primeiro:** a decisão do tick usa valores já atualizados — o agente reage à fome deste tick, não à do tick anterior.
- **Conclusão libera no mesmo tick:** uma ação que termina no passo 2 deixa `current_action` nulo, e o passo 3 escolhe a próxima ação no mesmo tick — não há tick ocioso entre ações.
- **Deltas começam no tick seguinte:** uma ação iniciada no passo 3 só recebe a primeira aplicação de deltas no tick posterior, porque o passo 2 já passou. Em ticks curtos o efeito é desprezível; em ticks de 1 min de jogo, entra na contabilidade fina.
- **Seleção e preempção são mutuamente exclusivas no tick:** agente ocioso seleciona; agente ocupado (ou com fila não vazia) verifica preempção. Uma ação recém-iniciada nunca é preemptada no tick em que começa.
- **Não há passo de "processar fila" no tick de referência:** a fila é propriedade do jogo ([§8.2](execution.md#82-fila-de-ações-actionqueue)); o motor apenas a consulta para saber se a autonomia está suspensa.

### 8.2 Fila de ações (`ActionQueue`)

- O jogador (ou diretor de jogo, ou script de missão) pode enfileirar ações; elas têm prioridade sobre a autonomia, **mas** considerações críticas podem interrompê-las ([§8.3](execution.md#83-interrupção-preempção)).
- **A fila é gerida pelo jogo, não pelo motor.** O motor nunca desenfileira: ele consulta a fila apenas para (a) suspender a seleção autônoma enquanto ela não estiver vazia e (b) aplicar a regra de preempção mais estrita a ações marcadas como enfileiradas pelo jogador. Promover a cabeça da fila para `current_action` — marcando-a como vinda do jogador — é responsabilidade de quem integra.
- `NarrativeCommitment` (ex.: aliança, romance, missão escoltada) bloqueia ações autônomas que o contradigam — como **filtro de elegibilidade** na seleção ([§6.3](selection.md#63-pseudocódigo-neutro-de-linguagem)): a ação contraditória nem vira candidata. O compromisso declara as tags que o contradizem; a janela de validade (`expires_at`) faz parte do modelo, mas **a expiração não é aplicada pelo motor** — cabe ao jogo remover compromissos vencidos do estado do agente. `[PREMISSA]` na janela.
- **Custos não são debitados pelo motor.** `cost` entra como filtro de elegibilidade (o que não se pode pagar não é candidato) e como penalidade de score ([§6.2](selection.md#62-função-de-utilidade)); o débito efetivo dos recursos — no início ou ao longo da execução — é responsabilidade do jogo, coerente com a statelessness do serviço.

### 8.3 Interrupção / preempção

```
funcao check_preemption(A):
    current = A.current_action
    se current == nulo: retornar
    se nenhuma consideration c com c.value < c.critical_threshold: retornar
    se current veio da fila do jogador e nenhuma c com c.value <= -90: retornar
    best = select_action(A)                             // seleção estocástica completa [§6.3](selection.md#63-pseudocódigo-neutro-de-linguagem)
    se best == nulo: retornar                           // sem alternativa elegível, a ação continua
    se best.utility > preemption_margin * current.continuation_utility:
        interrupt(current)                              // deltas parciais já aplicados se mantêm
        start_action(best)
```

Sutilezas normativas:

- **O gatilho é qualquer consideração crítica**, não necessariamente a que a nova ação atende. O agente interrompe o almoço porque `THREAT` despencou; o que ele faz em seguida é decidido pela seleção normal.
- **A substituta é escolhida pela seleção estocástica completa**, não por "o melhor candidato para a consideração crítica". Sob urgência o agente quase sempre troca para o melhor alívio disponível, mas a temperatura pode fazê-lo trocar para o segundo melhor — a preempção herda a imperfeição deliberada ([§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)).
- **`continuation_utility` é um retrato congelado** no instante em que a ação atual foi escolhida, não uma reavaliação. Uma ação escolhida em momento de desespero carrega um retrato alto e é difícil de desalojar depois; um plano B herdado da arbitragem de lote ([§8.5](execution.md#85-lote-de-decisão-duas-fases-e-arbitragem-de-disputa)) carrega sua utilidade menor e cede com facilidade. Se o retrato for ≤ 0, qualquer candidato de utilidade positiva vence a margem.
- **A comparação é estrita:** empatar com a margem não preempta — o empate favorece o compromisso.
- **Ações enfileiradas pelo jogador** só são preemptadas por colapso iminente (`value` ≤ -90, constante fixa da referência). `[PREMISSA]`
- `preemption_margin` default 1,5×; valor zero no tuning cai no default ([§6.3](selection.md#63-pseudocódigo-neutro-de-linguagem)).

### 8.4 Níveis de detalhe de simulação (LOD)

Agentes longe do jogador são simulados de forma simplificada (eventos agregados: trabalhou, viajou, fez inimigo, foi derrotado); ao retornar à vizinhança do jogador, o estado é **reconstruído de forma plausível** (quem acabou de jantar não pode reaparecer faminto; quem estava em batalha pode reaparecer ferido).

- **`FULL`:** loop de [§8.1](execution.md#81-loop-de-atualização-por-tick-por-agente-em-nível-full), tick fino (ex.: 1 s–1 min de jogo). `[PREMISSA]`
- **`SIMPLIFIED`:** tick grosso (ex.: 1 h de jogo); considerações avançam por modelo agregado; decisões são eventos discretos. `[PREMISSA]`
- **Transição `SIMPLIFIED` → `FULL`:** inicializar considerações com valores condizentes com a agenda simulada e a hora do dia. `[PREMISSA]` no mecanismo. A referência ilustra o padrão com reconstrução por eventos agregados: cada evento de refeição com horário anterior ou igual à hora atual restaura um montante fixo em `HUNGER` (com clamp nos limites da consideração); eventos posteriores à hora atual são ignorados; o log de eventos agregados é descartado na transição. O invariante é a plausibilidade (CA-11: refeição às 12h, retorno às 13h ⇒ `HUNGER` ≥ +30); os montantes por tipo de evento são definidos por jogo.

### 8.5 Lote de decisão: duas fases e arbitragem de disputa

Quando o serviço decide por milhares de agentes numa única chamada, decisões calculadas independentemente podem disputar o mesmo recurso: dois agentes pontuam a mesma cadeira sem ver a escolha um do outro. A resposta do lote precisa respeitar as vagas do mundo — **nenhum lote devolve mais agentes para um provedor ou para uma ação do que eles têm vagas** ([§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)) — sem sacrificar o paralelismo que faz o lote escalar. O desenho resolve as duas exigências separando o ciclo em duas fases.

**Fase 1 — pontuação paralela.** Cada agente é processado de forma totalmente independente: elegibilidade, score, top-K, sorteio estocástico e montagem da lista ordenada de preferências ([§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote)). O mundo é somente leitura nesta fase e cada agente escreve apenas na própria lista, portanto o trabalho se distribui por quantos núcleos houver, sem sincronização. O sorteio de cada agente usa uma fonte pseudoaleatória própria, derivada da seed do lote e da posição do agente no lote — a ordem em que os trabalhadores paralelos concluem não influencia nenhum sorteio.

**Fase 2 — reconciliação serial.** Encerrada a fase 1, uma única passada iterativa distribui as vagas disputadas. A reconciliação só precisa de três coisas por agente: identidade, posição e lista de preferências — as utilidades já cumpriram seu papel ordenando as listas. É barata porque cada lista tem no máximo `RECONCILIATION_TOP_K` entradas.

**Por que duas fases, e não pontuar já resolvendo.** A pontuação é o custo dominante e é paralela por construção: nenhum agente lê decisão de outro. A disputa é inerentemente global — vaga é recurso compartilhado — e não paraleliza sem locks ou rodadas de correção; mas é barata. Separar as fases deixa cada uma com a estrutura mais simples possível: primeiro um mapeamento puro, depois uma redução serial determinística.

**O critério de disputa é proximidade, não utilidade.** Quem está mais perto leva. A lógica é física, não de mérito: quem alcança o recurso antes fica com ele. Utilidade **não** é critério de desempate — "quem tem mais fome come primeiro" não existe no mundo real: fome não acelera ninguém. Registrar explicitamente, porque "maior utilidade ganha a disputa" é a proposta intuitiva que reaparece a cada revisão deste desenho — e está errada: ela faria o NPC mais necessitado teleportar prioridade para si, tornaria a distância irrelevante exatamente onde ela mais importa (recurso escasso disputado) e destruiria a legibilidade espacial da simulação (CA-15: o faminto distante perde para o saciado próximo).

**A rodada de reconciliação.**

```
funcao reconcile(agents, preferences) -> uma ActionInstance ou nulo por agente:
    cursor[i] = 0 e assignment[i] = indefinido, para todo agente i
    repetir:
        displaced = falso
        claims = agrupar por provedor os agentes com cursor[i] < |preferences[i]|;
                 cada um reivindica o candidato preferences[i][cursor[i]]
        para cada provedor com reivindicantes:
            ordenar reivindicantes por distância crescente ao provedor;
            desempate por ID do agente                        // ordem explícita e estável
            vagas_provider = capacity - |occupants|
            vagas_action[a]  = capacity(a) - occupancy(a)     // por ação; 0 = sem limite
            para cada reivindicante, na ordem:
                se vagas_provider > 0 e vagas_action[acao_do_candidato] != 0:
                    assignment[i] = cursor[i]; decrementar as duas vagas
                senao:
                    assignment[i] = indefinido; cursor[i]++; displaced = verdadeiro
    ate displaced == falso
```

Três propriedades fazem o laço correto:

- **O perdedor cai para o próximo candidato da própria lista** (CA-16) — nunca para uma lista alheia, nem para um candidato de que já foi desalojado. A queda pode criar disputa nova em outro provedor; daí a iteração.
- **Atribuições são provisórias até uma rodada sem desalojamentos.** Quem conseguiu vaga na rodada 1 pode perdê-la na rodada 2 para um contendente mais próximo que caiu em cascata sobre o mesmo recurso (CA-18; exemplo completo em [§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote)). Cada rodada reavalia todas as reivindicações em aberto, não apenas as novas.
- **O laço termina.** O cursor de um agente só anda para frente na própria lista: desalojado do candidato `i`, ele nunca mais aponta para `i`. Cada agente é desalojado de cada candidato no máximo uma vez, logo o total de desalojamentos é limitado pela soma dos tamanhos das listas — finita, porque cada lista tem no máximo `RECONCILIATION_TOP_K` entradas.

**Determinismo.** A ordem da reconciliação é explícita e estável — distância crescente, desempate por ID do agente — e cada agente aparece em exatamente uma lista de reivindicação por rodada, portanto nem a ordem de iteração sobre os provedores nem a ordem de conclusão dos trabalhadores da fase 1 afetam o resultado. Somada à fonte pseudoaleatória por agente (seed do lote + posição no lote), a garantia é: mesma entrada + mesma seed = mesma atribuição, inclusive nas disputas (CA-19).

**Esgotamento.** Agente cuja lista se esgota sem conseguir vaga sai sem ação: a resposta do lote nasce pré-preenchida com "sem ação" (`-1`) e só é sobrescrita pelas atribuições efetivas — o esgotamento é o caso normal de recurso escasso, não um erro.

**Statelessness.** A arbitragem acontece inteiramente dentro de uma chamada; o serviço não guarda mundo entre requisições. A decisão de um agente isolado é simplesmente um lote de tamanho 1: mesma pontuação, mesma reconciliação, nenhuma disputa.

---
