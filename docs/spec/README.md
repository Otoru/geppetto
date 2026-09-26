# Especificação Técnica — Arquitetura de IA Utilitária para NPCs (Utility AI com Affordances)

**Escopo:** arquitetura genérica de decisão autônoma para NPCs de qualquer gênero de jogo (RPG, survival, stealth, tático, imersivo, estratégia, simulação social, mundo aberto).
**Núcleo:** `considerations → response curves → utility → selection`. O ambiente anuncia ações; o agente pontua e escolhe — deliberadamente sem escolher sempre a melhor.
**Marcações:** `[PREMISSA]` indica decisão de engenharia arbitrária (valores default, forma de função, mecanismo de seleção). A proveniência desta spec está no [Apêndice A](provenance.md).
**Implementação de referência:** o projeto [geppetto](https://github.com/vitorhugo/geppetto) implementa esta especificação; este documento é versionado junto do código (ver [Apêndice A](provenance.md)).

---

## Índice

- [README.md](README.md) — visão geral, objetivos e glossário da especificação.
- [data-model.md](data-model.md) — modelo de dados e atualizadores de consideração.
- [affordances.md](affordances.md) — protocolo de anúncio de affordances.
- [selection.md](selection.md) — seleção de ação e imperfeição deliberada.
- [execution.md](execution.md) — ciclo de execução, fila, preempção e arbitragem de disputa.
- [genres.md](genres.md) — instanciações da arquitetura por gênero de jogo.
- [tuning.md](tuning.md) — parâmetros de tuning consolidados.
- [acceptance.md](acceptance.md) — critérios de aceitação CA-1 a CA-27.
- [provenance.md](provenance.md) — proveniência e limites da especificação.

---

## 1. Visão geral e objetivos

### 1.1 O que é o sistema

Um sistema de **agentes autônomos baseados em utilidade**. Cada agente (NPC) mantém um conjunto de **considerações** (`Consideration`) — sinais escalares normalizados que descrevem seu estado interno e sua leitura do mundo (fome, ameaça, munição, cobertura, vínculos, objetivos de facção, medo, fadiga…). As **entidades do mundo** atuam como **provedores de affordance** (`AffordanceProvider`): anunciam as ações que oferecem e os deltas que prometem sobre as considerações. Quando o agente decide, ele **pontua as ações anunciadas ao alcance** e escolhe uma das mais bem pontuadas — **não necessariamente a melhor**.

A inversão arquitetural central: o agente **não conhece** as entidades do mundo. Não existe código no agente do tipo "se com fome, procure geladeira" nem "se sob fogo, procure cobertura". O conhecimento sobre o que cada coisa oferece vive **nos provedores**. Novas entidades, itens, locais e eventos entram no sistema sem alterar o "cérebro" dos agentes.

### 1.2 Objetivos

1. **Autonomia plausível:** o agente decide sozinho em qualquer ambiente, com qualquer combinação de entidades, atores e situações.
2. **Extensibilidade:** adicionar um provedor de affordance novo (arma, abrigo, altar, ponto de patrulha, NPC conversável) não exige mudança no agente — basta o provedor declarar seus anúncios.
3. **Comportamento legível:** traços e preferências enviesam as escolhas de forma que um observador consiga *contar uma história* sobre por que o NPC fez aquilo.
4. **Emergência:** o sistema deve gerar situações inesperadas e memoráveis, não apenas comportamento correto.
5. **Escala:** suportar dezenas/centenas de agentes, simulando de forma barata os que estão longe do jogador (níveis de detalhe de simulação).

### 1.3 Não-objetivos (por que o agente NÃO deve ser ótimo)

- **Não maximizar eficiência.** Um NPC que executa sempre a política ótima (posicionamento perfeito, economia perfeita de recursos, rotina perfeita) elimina tensão, exploração e jogo.
- **Não ser previsível.** Seleção por argmax puro transforma NPCs em autômatos exploráveis: o jogador aprende a função e a manipula. Erro deliberado sustenta a ilusão de intenção.
- **Não ser correto demais.** Regras de comportamento perfeitamente seguidas produzem resultados previsíveis e sem graça; desvios ocasionais geram situações memoráveis.
- **Não simular tudo.** Uma boa simulação não precisa simular tudo; simular menos, nos lugares certos, melhora o jogo e o orçamento de CPU.
- **Não contradizer a narrativa do jogador.** A autonomia cuida da vida do NPC, mas não deve desfazer atos do jogador nem histórias em andamento (regra de improvisação: "aceite a ideia do jogador e continue a partir dela").

---

## 2. Glossário

| Termo | Definição |
|---|---|
| **Agente / NPC** (`Agent`) | Entidade autônoma com considerações, personalidade e fila de ações. |
| **Consideração** (`Consideration`) | Sinal escalar normalizado (tipicamente [0, 1] ou [-100, +100]) que alimenta a função de utilidade. Generaliza "necessidades": inclui estado fisiológico, ameaça, recursos, posicionamento, relações, objetivos, medo, estado de missão. |
| **Atualizador de consideração** (`ConsiderationUpdater`) | Regra que evolui uma consideração ao longo do tempo ou em resposta a eventos. O **decaimento temporal** é um tipo de atualizador — não o modelo inteiro. |
| **Curva de resposta** (`response_curve`) | Função que converte o valor bruto de uma consideração em **pressão** (peso) para a utilidade. |
| **Traço** (`Trait`) | Característica nomeada de personalidade (ex.: `cautious`, `sadistic`, `lazy`, `loyal`) que enviesa pontuações e pode criar ou modular considerações. |
| **Provedor de affordance** (`AffordanceProvider`) | Qualquer entidade do mundo que anuncia ações: objeto, local, zona, outro NPC, evento, item, ponto de patrulha, cobertura. |
| **Anúncio / ação anunciada** (`AdvertisedAction`) | Declaração, feita pelo provedor, de uma ação disponível + deltas prometidos sobre considerações + precondições. |
| **Utilidade / pontuação** (`utility`, calculada por `score_action()`) | Número calculado pelo agente para cada ação anunciada; base da seleção. |
| **Pressão / urgência** (`pressure`) | Peso de uma consideração após a curva de resposta; cresce conforme a situação se torna crítica. |
| **Seleção estocástica** | Escolha aleatória enviesada entre as ações mais bem pontuadas, em vez de argmax determinístico. |
| **Contexto situacional** (`Context`) | Modificador temporário de utilidade criado por local, papel social, estado de combate, missão ou facção. |
| **LOD de simulação** (`simulation_level`) | Agentes longe do jogador são simulados de forma simplificada (eventos agregados); ao voltar para perto, seu estado é reconstruído de forma plausível. |
| **Compromisso narrativo** (`NarrativeCommitment`) | Estado que a autonomia não deve destruir (ex.: aliança ou romance iniciado pelo jogador, missão em andamento). |

### 2.1 Convenção de nomenclatura

Todos os identificadores desta spec são em **inglês**, com o seguinte casing. Quem estender a spec (novas considerações, tags, parâmetros) deve seguir o mesmo padrão:

| Categoria | Convenção | Exemplos |
|---|---|---|
| Entidades / tipos / estruturas | PascalCase | `Agent`, `Consideration`, `AffordanceProvider`, `AdvertisedAction`, `Trait`, `ActionQueue` |
| Campos e parâmetros de tuning | snake_case | `decay_rate`, `preemption_margin`, `advertisement_radius`, `response_curve_exponent` |
| Valores de enum / constantes / nomes de consideração | SCREAMING_SNAKE_CASE | `HUNGER`, `ENERGY`, `THREAT`, `COVER`, `AMMO`, `MORALE`, `FULL`, `SIMPLIFIED` |
| Tags de ação | lowercase kebab-case | `social`, `leisure`, `offensive`, `defensive`, `stealth`, `break-convention` |
| Funções em pseudocódigo | snake_case | `score_action()`, `select_action()`, `update_considerations()` |

A prosa da spec permanece em português; apenas identificadores usam inglês.

---
