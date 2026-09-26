# Parâmetros de tuning consolidados

## 10. Parâmetros de tuning consolidados

Os defaults desta tabela são os **defaults embutidos do motor de referência**, verificados contra a implementação. Regra geral de configuração: **um campo de tuning omitido ou deixado em 0 não significa "zero" — significa "não configurado"**, e o motor substitui pelo default. Consequência prática: não é possível zerar `W_PRIORITY` ou `SELECTION_TEMPERATURE` pelo perfil; quem quer prioridade intrínseca irrelevante deve usar valores pequenos (ex.: 0,5), não 0.

| Parâmetro | Default | Faixa útil | Onde atua |
|---|---|---|---|
| Faixa de consideração (`value`) | [-100, +100] bipolar ou [0, 100] unipolar | convenção fixa | [3.1](data-model.md#31-consideration) |
| `base_weight` por consideração | sem default (obrigatório por consideração) | [0, 10] | [3.1](data-model.md#31-consideration) / [4.3](data-model.md#43-curvas-de-resposta-valor--pressão) |
| `response_curve_exponent` (convexidade da urgência) | 2,0 | [1, 4] | [4.3](data-model.md#43-curvas-de-resposta-valor--pressão) |
| `decay_rate` por consideração | sem default (0 = consideração parada) | 3–20 pts/h | [4.2](data-model.md#42-atualização-por-tick) |
| `critical_threshold` por consideração | sem default | [-100, +100]; bipolares tipicamente [-60, -30]; unipolares podem ser positivos | [4.2](data-model.md#42-atualização-por-tick) / [8.3](execution.md#83-interrupção--preempção) |
| `W_PRIORITY` (peso da prioridade intrínseca) | 5 | [0, 20] | [6.2](selection.md#62-função-de-utilidade) |
| `DISTANCE_REFERENCE` (distância de referência) | 10 m | [2, 50] | [6.2](selection.md#62-função-de-utilidade) |
| `SELECTION_TOP_K` (candidatos no sorteio) | 3 | [1, 10] | [6.3](selection.md#63-pseudocódigo-neutro-de-linguagem) |
| `SELECTION_TEMPERATURE` (temperatura softmax) | 1,0 (0,5–1,2 nos perfis de gênero, [§9](genres.md#9-instanciação-por-gênero)) | [0,1, 5] | [7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) |
| `preemption_margin` | 1,5× | [1,0, 3,0] | [8.3](execution.md#83-interrupção--preempção) |
| `convention_break_probability` | 0,15 | [0, 1] | [7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) |
| `perception_noise` (σ) | 5 pts | [0, 30] | [7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las) |
| Multiplicadores de traço (`Trait.modifiers`) | sem default | [0, 5]; típico 0,5–2,0 | [3.2](data-model.md#32-trait-e-personality) / [6.4](selection.md#64-contextos-situacionais) |
| Tick `FULL` / `SIMPLIFIED` | 1 min / 1 h de jogo | > 0 | [8.4](execution.md#84-níveis-de-detalhe-de-simulação-lod) |
| `capacity` de provedor | **sem default — o cliente declara por provedor a cada requisição** | [0, ∞); 0 bloqueia todos | [3.4](data-model.md#34-affordanceprovider) |
| `capacity` de ação | 0 (**sem limite** — [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero)) | [0, ∞) | [3.5](data-model.md#35-advertisedaction) / [5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero) |
| `occupancy` de ação | 0 (ninguém executando) | [0, `capacity`] | [3.5](data-model.md#35-advertisedaction) / [5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero) |
| `RECONCILIATION_TOP_K` (candidatos retidos por agente) | 5 | [1, ∞) | [6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote) |
| `advertisement_radius` | **sem default — cada ação declara o seu** | [1, 100] m | [3.5](data-model.md#35-advertisedaction) |

### 10.1 Pressão e urgência (por consideração)

**`base_weight`** — escala linear da pressão: `pressure = base_weight × curva(valor)`. É como o perfil expressa "sobrevivência importa mais que conforto" sem tocar nas curvas. **Aumentar** faz a consideração dominar a soma de utilidade mesmo longe do fundo (NPC obcecado); **diminuir** a torna um desempate sutil. Os perfis de referência usam 1 para luxo (`FUN`), 2–3 para rotina (`DUTY`, `ENERGY`), 4–5 para sobrevivência e perigo (`HUNGER` em survival, `THREAT`, `SECURITY`).

**`response_curve_exponent`** — convexidade da curva de urgência. Com 2 (default), uma consideração a -80 pesa ~4× mais que a -20: o NPC convive com o problema até ele ficar sério, e então a urgência explode. **Aumentar** (→ 4) concentra ainda mais a pressão no fundo: o NPC ignora a necessidade por mais tempo e reage de forma mais abrupta quando reage — bom para drama, ruim para credibilidade fisiológica. **Diminuir** (→ 1) aproxima do linear: reação gradual e previsível, sem "ponto de virada". Considerações individuais podem sobrescrever o expoente do perfil.

**`decay_rate`** — velocidade do relógio biológico, em pontos por hora de jogo. **Aumentar** encurta o ciclo da necessidade: mais viagens à geladeira/fogueira/cama por dia de jogo, ritmo mais frenético. **Diminuir** alonga o ciclo e dilui a simulação. Referência dos perfis: 8–12,5 pts/h cobrem 2–3 refeições e 1 descanso longo por dia de 24 h de jogo. Cuidado: `decay_rate` omitido vale 0 — a consideração **nunca se move**, o que é quase sempre um erro de configuração silencioso.

**`critical_threshold`** — o piso que autoriza preempção: só quando alguma consideração está **abaixo** dele o agente pode interromper a ação em curso. **Subir** o limiar (ex.: de -50 para -30) produz NPCs cautelosos, que largam o que estão fazendo cedo; **descer** (→ -80) produz NPCs que só reagem no desespero — cenas de risco e colapso. Em considerações unipolares o limiar pode ser positivo: munição [0, 100] com limiar 10 lê-se como "pente quase vazio é emergência". Há ainda um piso fixo de colapso iminente em -90 (não configurável): abaixo dele, até ações enfileiradas pelo jogador podem ser interrompidas.

### 10.2 Função de utilidade

**`W_PRIORITY`** — quanto o `intrinsic_priority` de um anúncio pesa na utilidade. É a alavanca entre "NPC movido a necessidades e personalidade" e "NPC movido a ordens e papéis". **Aumentar** (→ 20): prioridades estruturais (ordens diretas, deveres de missão) atropelam fome e preferência pessoal — NPCs disciplinados. **Diminuir** (→ 1–2, como nos perfis social e de vila): a vida interna manda; ordens competem de igual para igual com uma boa conversa. Lembrete: 0 não zera — cai no default 5.

**`DISTANCE_REFERENCE`** — a distância em que o multiplicador de distância cai para 0,5: `M_distance = 1 / (1 + d / DISTANCE_REFERENCE)`. **Aumentar** (→ 50): distância quase não desconta, NPCs aceitam atravessar o mapa por um ganho marginal — decisões "espertas", mundo que parece pequeno. **Diminuir** (→ 2): comportamento fortemente local, NPCs que preferem qualquer opção medíocre próxima — mundo grande, deambulação crível, mas risco de NPC "míope" que morre de fome ao lado da comida.

### 10.3 Seleção estocástica

**`SELECTION_TOP_K`** — quantos dos candidatos mais bem pontuados entram no sorteio softmax. **Aumentar** (→ 10): opções piores passam a ter chance real — mais variedade de comportamento entre NPCs idênticos, mas escolhas visivelmente ruins vazam para a tela. **Diminuir** (→ 1): o sorteio degenera para o melhor candidato — comportamento ótimo e idêntico entre NPCs, morte da variedade. O default 3 é o ponto onde o segundo colocado ainda aparece com frequência legível.

**`SELECTION_TEMPERATURE`** — o botão mestre da imperfeição. Softmax: `P(i) ∝ e^(u_i / temperatura)`. **Alta** (1,2–1,5): as diferenças de utilidade se achatam, o NPC erra mais — e cada erro legível vira história ("ele foi dançar com fome porque quis"); NPCs imprevisíveis, vilas vivas. **Baixa** (0,3–0,6): o melhor candidato ganha quase sempre; NPCs competentes, frios, previsíveis — é o que vende um gênero tático. **Próxima de 0** vira argmax disfarçado: proibido em produção, porque destrói a ilusão de vida e abre exploração pelo jogador. Lembrete: 0 não é "zero absoluto" — cai no default 1,0.

### 10.4 Preempção

**`preemption_margin`** — fator pelo qual a utilidade do melhor candidato disponível deve superar a da ação em curso para interrompê-la; a comparação é **estrita** (empatar com a margem não preempta). **Aumentar** (→ 2–3): NPC "teimoso", termina o que começou mesmo com a casa pegando fogo moderado — cenas cômicas e dramáticas, mas risco de parecer quebrado. **Diminuir** (→ 1,1–1,3): NPC reativo, troca de ação ao primeiro sinal de perigo — essencial em combate, robótico em cena social. A preempção só é avaliada quando alguma consideração está abaixo do `critical_threshold`; sem urgência, a ação em curso nunca é interrompida.

### 10.5 Imperfeição situacional

**`convention_break_probability`** — probabilidade de o NPC ignorar uma convenção social/tática "correta". **Aumentar**: mais gafes, quebras de protocolo e situações memoráveis; em excesso, o mundo perde coerência. **Zerar**: NPCs sempre educados e previsíveis.

**`perception_noise`** — desvio-padrão do ruído aplicado a considerações `perception_driven`. **Aumentar** (→ 15–30): erros críveis de leitura do mundo — o guarda que não vê o intruso óbvio, o aldeão que entra em pânico sem motivo. É o canal certo de imperfeição para gêneros que exigem decisões competentes (stealth): o erro mora na percepção, não na escolha. **Zerar**: percepção perfeita, reação instantânea e infalível a ameaças.

**Multiplicadores de traço** — viés permanente por tag de ação. Típico 0,5–2,0; acima de 3 (ex.: `cowardly` ×3 em `flee`) o traço vira caricatura dominante — desejável para NPCs cômicos ou de missão, perigoso para o elenco inteiro.

### 10.6 LOD (níveis de detalhe)

**Tick `FULL` (1 min de jogo) / `SIMPLIFIED` (1 h de jogo)** — granularidade temporal dos dois níveis de simulação. Encurtar o tick `FULL` refina reações e encarece o lote; alongar o tick `SIMPLIFIED` barateia multidões distantes ao custo de reconstruções de estado mais grosseiras na volta ao modo `FULL` ([§8.4](execution.md#84-níveis-de-detalhe-de-simulação-lod)).

### 10.7 Capacidade e arbitragem de disputa

**`capacity` de provedor** — vagas simultâneas do provedor, **declarada pelo cliente a cada requisição**; o motor não tem default. Diferente da capacidade de ação, **0 bloqueia todos**: um provedor com `capacity` 0 não recebe ninguém. Convenção recomendada: 1 para recursos de uso individual (cama, serra, privada), N para recursos coletivos (mesa de jantar, praça).

**`capacity` de ação** — vagas simultâneas **daquela ação específica** dentro do provedor. **0 = sem limite próprio** (apenas a capacidade do provedor manda) — semântica deliberada, oposta à do provedor, porque um campo numérico ausente no protocolo chega como 0 e clientes antigos nunca o enviam. Uma bancada com 4 vagas cuja serra tem `capacity` 1 admite no máximo 4 agentes, dos quais exatamente 1 serrando. **O nível mais restritivo sempre vence.**

**`occupancy` de ação** — quantos agentes o cliente informa que **já estão** executando a ação no início do tick. Consome vagas antes de qualquer decisão nova: ação com `capacity` 2 e `occupancy` 1 admite só mais 1; com `occupancy` = `capacity`, a ação nem entra na lista de candidatos daquele tick. Serve para o cliente manter continuidade entre ticks sem que o motor guarde estado.

**`RECONCILIATION_TOP_K`** — tamanho da lista de preferência que cada agente retém para a arbitragem: a escolha estocástica seguida dos planos B em ordem decrescente de utilidade. **Aumentar**: menos agentes ficam sem ação em cenas disputadas (cada perdedor tem mais para onde cair), ao custo de memória por agente — em lotes de dezenas de milhares de NPCs, listas longas estouram o orçamento. **Diminuir** (→ 1–2): arbitragem barata, mas perdedores de disputa desistem mais cedo e ficam ociosos. 0 ou omitido = default 5.

**`advertisement_radius`** — alcance em que o anúncio é visível, **por ação, sem default**: cada anúncio declara o seu. **Aumentar**: NPCs "sabem" de oportunidades distantes — decisões mais espertas, menos deambulação, mundo menor. **Diminuir**: descoberta local, exploração crível; raio 0 restringe o anúncio a agentes exatamente na posição do provedor (na prática, invisível). Perfis típicos: raio grande para recursos de exploração (uma clareira com frutas "chama" de longe), pequeno para estações e interações íntimas (conversa exige proximidade).

---
