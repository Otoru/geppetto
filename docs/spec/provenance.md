# Proveniência e limites desta especificação

## Apêndice A — Proveniência e limites desta especificação

**Origem.** Esta especificação nasceu da análise de um vídeo divulgativo (canal Vertex, pt-BR, ~15 min) sobre a inteligência artificial de um jogo de simulação de vida, transcrita automaticamente pelo YouTube. O vídeo descrevia, em linguagem informal: necessidades que decaem, objetos que anunciam o que oferecem, seleção por pontuação com aleatoriedade deliberada, traços de personalidade que criam motivações, e simulação em dois níveis de detalhe.

**Generalização.** A versão original desta spec era específica daquele domínio; esta versão abstrai o núcleo (`considerations → response curves → utility → selection`; provedores de affordance) para servir a qualquer gênero de jogo. Necessidades fisiológicas com decaimento são aqui apenas uma instanciação entre várias ([§4.1](data-model.md#41-tipos-de-atualizador), [§9](genres.md#9-instanciação-por-gênero)).

**O que este documento NÃO é.** Nada aqui é afirmação sobre o código, parâmetros ou arquitetura reais daquele jogo ou de seu estúdio. Todos os números, fórmulas, mecanismos de seleção e estruturas de dados são decisões de engenharia desta spec (`[PREMISSA]`), plausíveis mas inventadas.

**Versionamento.** Este documento viveu originalmente fora de qualquer repositório. A partir da versão que introduz [§5.4](affordances.md#54-capacidade-em-dois-níveis-e-a-semântica-do-zero) e [§6.5](selection.md#65-arbitragem-de-disputa-dentro-do-lote), ele passa a ser versionado junto do código no repositório do geppetto (`docs/spec/README.md`), implementação de referência desta arquitetura — a cópia no repositório é a canônica. As seções novas descrevem comportamento já implementado e testado (CA-15 em diante), não aspiração.

**Notas de interpretação da transcrição** (legendas automáticas; trechos possivelmente corrompidos, interpretados pelo contexto):

1. **"Simless" → "Simlish"**: idioma inventado dos personagens; o contexto (fala ambígua, emoção sem significado exato) confirma. O conceito subjacente — ambiguidade deliberada como espaço para a imaginação do jogador — sobrevive na diretriz de "variedade legível" ([§7](selection.md#7-fontes-deliberadas-de-imperfeição-e-como-sintonizá-las)).
2. **"Sim and" → "SimAnt"**: jogo anterior do mesmo criador, sobre formigas atraídas por feromônios; citado como origem histórica da ideia de agentes atraídos por sinais do ambiente — análoga ao protocolo de anúncios ([§5](affordances.md#5-protocolo-de-anúncio-de-affordances)).
3. **"Anúncios"**: mantido do original; corresponde ao conceito conhecido de *advertisement* em smart objects, aqui generalizado para affordances (`AdvertisedAction`).
4. **"IA utilitária"**: o vídeo usa o termo corretamente (*utility AI*); esta spec o adota como nome da arquitetura.

