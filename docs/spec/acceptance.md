# Critérios de aceitação (observáveis e testáveis)

## 11. Critérios de aceitação (observáveis e testáveis)

1. **CA-1 (Atualizadores):** com nenhuma ação executada, uma consideração `linear_decay` configurada com `decay_rate` = 10 pts/h vai de +100 a -100 em ~20 h de jogo (±10%). Considerações `event_driven` não se movem sem eventos; considerações `perception_driven` respondem a mudanças do ambiente em ≤ 1 tick.
2. **CA-2 (Publicidade / extensibilidade):** instanciar um provedor novo com anúncio `{rest: ENERGY +80}` em ambiente previamente sem fonte de `ENERGY` faz agentes com `ENERGY` < `critical_threshold` usá-lo **sem nenhuma alteração no código do agente**.
3. **CA-3 (Urgência domina):** agente com `HUNGER` = -70 e `FUN` = -10, diante de provedores de comida e lazer equidistantes com deltas equivalentes, escolhe comer em ≥ 90% de 200 amostragens (com `SELECTION_TEMPERATURE` default).
4. **CA-4 (Saturação):** agente com `ENERGY` = +95 nunca escolhe `rest` quando existe qualquer outra ação com utilidade > 0 disponível.
5. **CA-5 (Personalidade):** dois agentes idênticos exceto `preference(book)` = +8 vs `preference(arena)` = +8, com `FUN` = -50 e ambos os provedores disponíveis, divergem na escolha em ≥ 80% das amostragens.
6. **CA-6 (Imperfeição):** em 1.000 decisões com um candidato de utilidade destacada (2× o segundo), a melhor opção é escolhida em 60–90% dos casos — nunca 100% (`SELECTION_TEMPERATURE` > 0 garantido) e nunca ≤ 1/`SELECTION_TOP_K` (não é aleatório puro).
7. **CA-7 (Distância):** duplicar a distância de um provedor reduz sua utilidade pelo fator previsto por `M_distance` ([§6.2](selection.md#62-função-de-utilidade)), mantidos os demais termos.
8. **CA-8 (Precondições):** ação cuja `preconditions` (skill, facção, item, estado) não é satisfeita jamais aparece entre os candidatos pontuados; ao ganhar a skill/facção, o anúncio passa a ser elegível sem reiniciar o agente.
9. **CA-9 (Contexto):** agente que entra num local com `Context` ×2,0 em tag `exercise` aumenta a frequência de ações dessa tag em ≥ 2× durante a permanência, vs. baseline fora dele.
10. **CA-10 (Preempção):** agente executando ação de lazer com uma consideração cruzando o `critical_threshold` interrompe a ação e busca provedor que a atenda em ≤ 1 tick de decisão.
11. **CA-11 (LOD):** agente em `SIMPLIFIED` cuja agenda agregada incluiu "refeição" às 12h e retorna ao modo `FULL` às 13h reaparece com `HUNGER` ≥ +30 (estado plausível, não faminto).
12. **CA-12 (Compromisso narrativo):** durante um `NarrativeCommitment` ativo (ex.: romance, escolta), o agente não inicia autonomamente ações que o contradigam.
13. **CA-13 (Generalidade):** o mesmo motor, sem alteração de código, executa as quatro configurações de [§9](genres.md#9-instanciação-por-gênero) apenas trocando tabelas de considerações, provedores, tags e parâmetros.
14. **CA-14 (Deltas mistos):** ação com deltas {+`FUN` 40, −`ENERGY` 30} é escolhida por agente entediado e descansado, e evitada por agente exausto e entediado, em ≥ 80% das amostragens de cada caso.
15. **CA-15 (Arbitragem por proximidade):** dois agentes disputando um provedor de capacidade 1 no mesmo lote: exatamente um é atribuído, e é o mais próximo — mesmo quando o mais distante tem utilidade estritamente maior (ex.: `HUNGER` -95 longe vs. `HUNGER` -10 perto; o faminto perde).
16. **CA-16 (Plano B após derrota):** o perdedor de uma disputa recebe o próximo candidato da própria lista de preferência; esgotada a lista, sai sem ação (`-1`).
17. **CA-17 (Capacidade maior que 1):** provedor com N vagas livres recebe exatamente os N contendores mais próximos; os demais caem para plano B ou `-1`.
18. **CA-18 (Cascata e estabilização):** a queda de um agente para seu plano B pode desalojar um agente mais distante já posicionado ali; a reconciliação itera até estabilizar, e o resultado final respeita todas as vagas.
19. **CA-19 (Determinismo da arbitragem):** mesma entrada e mesma seed produzem exatamente a mesma atribuição, inclusive com disputas nos dois níveis de capacidade.
20. **CA-20 (Invariante de vagas do provedor):** em nenhum resultado a contagem de agentes atribuídos a um provedor excede `capacity - |occupants|` — inclusive quando o cliente reporta ocupação parcial.
21. **CA-21 (Primeira preferência estocástica):** a arbitragem não vira argmax: a primeira preferência de cada agente é idêntica à que a seleção estocástica ([§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)) produziria sem arbitragem, dada a mesma seed.
22. **CA-22 (Capacidade por ação):** ação com `capacity` C admite no máximo C agentes por lote, mesmo com vagas sobrando no provedor (bancada com 4 vagas e serra com 1: exatamente 1 serra, no máximo 4 no total).
23. **CA-23 (Zero é ilimitado):** ação com `capacity` 0 — ou campo ausente na requisição — não impõe limite próprio e jamais bloqueia candidatos; vale apenas o limite do provedor.
24. **CA-24 (O mais restritivo manda):** ação com `capacity` 3 num provedor com `capacity` 1 admite exatamente 1 agente.
25. **CA-25 (Ocupação por ação):** `occupancy` informada pelo cliente consome vagas da ação antes do lote: ação com `capacity` 2 e `occupancy` 1 admite só mais 1 agente; ação com `occupancy` = `capacity` não é elegível para ninguém naquele tick.
26. **CA-26 (Invariante dos dois níveis):** em nenhum resultado a contagem por ação excede a `capacity` da ação (quando > 0), nem a contagem por provedor excede a do provedor.
27. **CA-27 (Candidatos retidos):** a lista de preferência retida por agente tem no máximo `RECONCILIATION_TOP_K` entradas — a escolha estocástica seguida dos fallbacks em ordem decrescente de utilidade; `RECONCILIATION_TOP_K` = 0 no tuning usa o default (5).

---
