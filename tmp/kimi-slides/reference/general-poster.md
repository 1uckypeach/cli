# 一般信息图表和海报

仅当用户明确请求信息图、海报或高度视觉化的单页设计时才使用本指南。请勿针对普通 PPT 请求加载本指南。

本指南仅定义了设计决策过程和质量基线；它没有规定固定的配色方案、字体、地图、时间线、网格或页面骨架。根据主题、受众、材料、使用场景独立设计；永远不要仅仅因为示例更详细而默认复制它。

## 生产流程

### 1.了解主题和沟通目标

首先确认：

- 这张海报应该让读者看到、理解、记住或做些什么。
- 主要受众、观看距离、显示介质和可用阅读时间。
- 用户明确要求的尺寸、长宽比、颜色、样式、内容和输出格式。
- 材料真正需要表达的核心关系：时间、空间、过程、对比、层次、物体结构或纯粹的精神/氛围传达。
- 现有的图像、数据和可验证的事实是否足以支持所选的表达形式。

### 2. 首先探索不同的方向，然后选择一个

在编写任何 `.page` 文件或放置元素之前，请在内部形成 2-3 个具有实质性差异的视觉方向。它们至少应在以下两项方面有所不同：

- 主要视觉焦点：图像、版式、对象、数据、地图/关系图或抽象图形。
- 信息结构：线性叙述、径向枢纽、局部缩放、分层切片、并置比较、图像矩阵或单个场景。
- 空间组织：全出血、离轴、对称、高密度、大量留白或分区结构。

至少一个候选方向应该将整个画布视为一个视野、场景或对象关系，而不是首先将内容分割成宽度和高度相等的容器。卡片可能会出现在候选者中，但不能仅仅因为信息有三或四个项目就自动成为默认卡片。

在最终确定方向之前，请比较哪种主要视觉媒体最能承载内容。当人物、文物、材料、环境或事件场景是该主题的重要证据时，实际检查至少 2 个在构图或视觉语言上存在实质性差异的现有图像、搜索结果、生成的图像、校样或参考屏幕截图；当流程、层次结构、系统、传播、因果关系、对比或空间关系处于中心位置时，请比较至少 2 个具有明显不同结构的可编辑矢量或类似 SmartArt 的图形草稿。在比较主题可识别性、缩略图可记忆性、信息容量、构图共存性和可编辑性后，选择摄影/真实对象图像、生成的图像、自定义矢量、结构化图形或混合解决方案。不要仅仅因为 PPTD 矢量更容易实现或因为搜索/生成资产需要时间而默认使用纯矢量，并且不要仅仅为了“拥有资产”而强制使用与内容无关的图像。

根据主题特征、用户语气、资产质量和主要阅读任务选择一个方向。您不需要提供三种设计，但不要在看到第一个可行选项时就开始构建。

可用的语法包括但不限于排版主导、摄影主导、对象/部分注释、档案拼贴、数据字段、空间地图或关系图。这些是思考的角度，而不是必须套用的模板。

### 3.在项目中写入`style.md`

对于新创建的海报/信息图，请在写入 `.page` 文件之前在 PPTD 项目根目录中创建一个简短的 `style.md`。它是当前任务的设计概要——不是新的技能文件，不是硬性 CLI 门，也不是额外的可交付成果。

在最终确定 `style.md` 之前，请检查用户的资源、实际图像候选以及 `reference/fonts.md` 中可用的字体。保持内容简短、具体且可操作；它通常包括：

```markdown
# Style Brief
- Goal and audience:
- Output scenario and canvas:
- Core message in one sentence:
- Chosen visual concept/metaphor:
- Primary visual focus:
- Poster-level visual move (why this is not an ordinary content page):
- Visual DNA (background/material, typographic/geometric temperament, how images coexist with structured information):
- Primary visual medium and asset/SmartArt-like figure candidates compared, with reasons for the choice:
- Reading order and composition:
- Color scheme and each color's role:
- Chinese and English fonts, size hierarchy, and usage:
- Image selection, cropping, and treatment:
- Graphics/chart/connector language (if needed):
- Information density, whitespace, and alignment:
- Tropes to avoid:
```

- 解释为什么选择这个方向，并用一句话记录没有选择其他方向的主要原因；还要说明所选方向最有可能退化为哪种比喻（例如，普通的技术报告、复古的宣传页面、商业促销或统一的卡墙），以便在屏幕截图审核时进行检查。
- 当用户已经提供了完整的设计系统、模板或强风格约束时，`style.md`仅将它们转换为可操作的决策；不要发明与用户要求相冲突的新样式。
- 在查看实际图像或发现资产限制后，您可以在细化页面之前更新一次`style.md`。不要将其变成冗长的流程日志。
- 对现有工件进行本地编辑时，不强制创建新的 `style.md`；仅在重新设计整体视觉效果时才需要它。

## 画布比例和方向

首先确定输出场景和画布，然后设计网格、字体大小和内容结构。不要默认将海报等同于纵向，也不要默认继承PPT的16:9。

1. 当用户明确指定尺寸、方向或比例时，严格使用用户的要求。
2、当用户没有指定时，根据部署场景、信息丰富程度、主要阅读顺序、关系结构自行选择。
3. 当信息过多时，优先考虑提炼、分层或拆分成几页；不要将所有内容都塞进一个字体非常小的页面中。
4、选择好画布后，直接按照该比例进行构图；不要先以 16:9 的比例构建它，然后将整个内容拉伸或裁剪为另一个比例。

|常用比例| PPTD参考尺寸|常见场景|
|---|---:|---|
| 9:16 肖像 | `[540, 960]` |移动阅读、社交媒体、垂直叙事|
| 3:4 肖像 | `[720, 960]` |类似印刷品的海报、人物/物体英雄视觉效果 |
| 1:1 平方| `[720, 720]` |社交媒体、中心结构、适度的信息量 |
| 4:3 风景 | `[720, 540]` |更高密度的文本图像内容、文档嵌入 |
| 16:9 风景 | `[960, 540]` |宽屏显示、水平关系和并行多列布局 |

PPTD画布可以使用任何`[width, height]`；表中的尺寸仅供参考，并非列举限制。

## 内容关系和图形选择

首先用一句话陈述页面的核心信息，然后选择最能表达该信息的图形。仅当关系本身是核心时才使用结构图；不要仅仅为了看起来丰富而添加地图、时间线、路线、流程图、矩阵或索引。平行关系并不自动等于卡片：首先考虑同一场景中的对象组、图像/符号序列、比例差异、空间相邻性、色带、规则线或印刷节奏。

当核心信息是流程、层次结构、系统、传播、因果关系、对比或空间关系时，将结构化图形（类似 SmartArt 的图形）、地图、路线或方法图视为主要视觉材料。所选择的结构图应占据足够的面积并参与整个页面的构成，尽可能使用可编辑的PPTD元素来实现；生成本机 OOXML SmartArt 对象不是必需的，但不要仅仅为了“可编辑性”而让它退化为一排圆角框、通用图标和细箭头。它可以与主要物体、场景图像、特写细节和锚定字幕一起形成一个连续的视野。

|核心关系|可能的表达 |
|---|---|
|时代演变，阶段变迁|时间表、里程碑、之前/之后状态 |
|路径、传播、迁移 |地图、路线、流量或网络 |
|步骤、机制、方法|流程、泳道、输入-流程-输出 |
|物体或观点之间的差异|并列比较、矩阵、尺度轴|
|层次结构、组成、系统 |树层次结构、体系结构、分层部分 |
|实物、产品、纸质方法详情|对象注释、局部缩放、视觉解剖 |
|多个事实或样本 |图像矩阵、分类图、数据字段 |
|情感、想法、态度|单一强烈的图像、纯粹的印刷构图、抽象的形式 |

- 当用户明确请求“地图”时，提供可以建立空间关系的地理基础或轮廓；不要将抽象折线冒充为地图。
- 对于路线、传播和迁移，您可以根据语义在全图、本地定位器、放射状网络、陆海分层或多个分支路线中自行选择；不要默认为细水平线。
- 对于架构图、UML、纸质方法图，在进行视觉美化之前，首先要确保节点、界面、方向、分组、图例正确。

## 设计自由度和质量基线

- 颜色、字体、图像处理和构图来自主题和 `style.md`。除非用户明确要求，否则不要默认使用黑-白-银-灰色、单点荧光口音、巨型裁剪类型、档案微型标签或细线时间线。
- 海报应首先建立一种占主导地位的视觉语法，并辅之以其他元素。不要为了显得“丰富”而一次性将英雄图像、巨型字体、地图、时间线、图标矩阵和数据图表塞满。
- 建立至少一个视觉记忆点，该点在整页缩略图或远处仍可识别，由英雄图像、巨型字体、大型物体、全出血色域、场景或核心关系图承载。如果缩略图仅显示一个标题和一组常规框，请重新构图。
- 当连续制作不同主题的多件作品时，不要默认重复使用前一件作品的主要构图、强调色角色、图像比例和线性图形。
- 网格、空白、卡片、拼贴、不对称或全出血都是可选方式，而不是默认的正确答案。避免将标题下方的每一类内容都塞进带有圆角、轮廓或阴影的等宽、等高矩形中，形成“三卡/四卡+CTA”幻灯片或应用程序界面骨架。仅当容器本身语义明确且不会形成统一的卡墙时才使用容器；否则更喜欢使用布局区域、比例、色块、规则线、空间邻接、对象组或印刷节奏来组织内容。
- 一个页面可能只有很少的文字，或者可能有几个高密度的模块；但核心信息、主要视觉焦点和阅读路径必须清晰。主要通过比例层次结构、局部缩放、标题和空间关系来组织丰富的内容；不要用重复的卡片或过大的填充来将稀疏的内容伪装成丰富的内容，也不要将大量的事实压缩成难以阅读的小文本。
- 正文、注释、图例和来源必须在实际输出场景中可读。当信息过多时，剪切、分层或分页；不要用很小的字体来解决这个问题。
- 数据、事实、日期、地点、标签、单位和来源必须忠实于用户的材料或可验证的信息。对于无法确认的精确数字，使用定性陈述、近似值或省略；不要为了“档案感”而捏造统计数据、坐标、发行号或出版细节。

### 很少的镜头：多个完整的摘要和匿名的视觉移动片段

以下完整示例用于扩展您可以借鉴的设计动作、主要视觉媒体和风险意识；它们不是固定的视觉系列、主题映射或等待选择的模板。实际简报的概念名称、视觉 DNA、颜色、字体、画布和构图必须根据当前主题、资产和阅读关系重新得出；您可以在样本中重新组合一些兼容的动作，或者完全设计自己的动作 - 不要只选择最相似的标签并复制整个集合。

#### 海报编辑

```markdown
# Style Brief — Poster Editorial
- Goal and audience: designed for quick browsing and short dwell; make the subject memorable first, then let readers discover archival details.
- Output scenario and canvas: choose landscape or portrait based on the actual medium; do not take a 16:9 layout and crop it.
- Core message: string together the object, time, and context with one judgment that takes a position; do not use section-style empty titles.
- Visual concept: "a contemporary, editorially curated object archive"; calm, sharp, with print tension, without imitating a real magazine cover.
- Primary visual focus: choose only one of three as the protagonist — a high-contrast object image, an object silhouette, or giant type.
- Reading order and composition: use a rigorous grid with a clear dense-versus-sparse counterpoint; the title, exhibit item, and micro captions form three scales, but not every poster is required to have a route or timeline.
- Color: Editorial White `#F7F6F2` and Deep Black `#0B0B0B` carry the main relationship; silver-gray is used only for secondary information; choose at most one small-area accent from Signal Red `#F20505`, Fluoro Lime `#D6FF00`, or Acid Orange `#FF4A1C`.
- Fonts: the main title uses 得意黑/阿里妈妈数黑体 with HedvigLettersSans; the body uses MiSans/Liter; at most three font families, and the numbering font does not carry body text.
- Image treatment: prefer images with strong contours, close-ups, and room for bold cropping; they may be converted to black and white or desaturated, but keep the object's recognition points.
- Graphic language: use hairlines, short labels, and small nodes only when content relationships need them; maps must have a spatial base, and data charts must have real data.
- Information density: keep one clear visual breathing area; archival details may be dense, but body text and captions must be readable at the actual output size.
- Must avoid: rounded-card walls, evenly distributed accent colors, fake logos/issue numbers/archives, and stacking giant type, a map, a timeline, and an icon matrix all at once just to seem rich.
- Reason for choosing: when the subject needs to be highly memorable while retaining traceable details, this direction is superior to a generic commercial infographic.
```

#### 几何构图

```markdown
# Style Brief — Geometric Composition
- Visual concept: translate the core concept into tension among circles, squares, lines, proportions, and glyphs; do not use geometric blocks to decorate empty space.
- Primary visual focus: one geometric relationship or glyph composition occupying the main area, not multiple evenly distributed colored rectangles.
- Poster-level visual move: let the title pass through, embed into, or cut across the main geometry, so text and composition together become the image, instead of the title hovering above the content.
- Visual DNA: decide color roles and typographic temperament from the subject; establish order with proportion, axes, numbering, and scale, with factual information taking its place along the geometric relationships.
- Information organization: bind concept differences, proportions, stages, or a work index to shape relationships; secondary material is tucked into the edges or a narrow index band.
- Must avoid: evenly tiling primary colors, a children's-building-block feel, random circles and squares unrelated to the content, and degenerating the geometric composition into a colored card wall.
```

#### 实质性证据

```markdown
# Style Brief — Material Evidence
- Visual concept: treat materials, artifacts, surface traces, or the making process as evidence, letting tactile quality and provenance relationships tell the content together.
- Primary visual focus: a sufficiently large macro, scan, section, or artifact detail; other images relate to it as slices, sequences, or provenance evidence.
- Poster-level visual move: let the texture or object span the main extent of the canvas, with captions anchored directly to visible details; do not shrink high-quality assets into specimen cards.
- Visual DNA: extract color, grain, and line language from the actual object; fonts and labels serve the material's temperament; maps, genealogies, or processes appear only when relationships need them.
- Information organization: organize facts through source labels, local zooms, material genealogy, craft steps, or propagation paths, keeping one clear breathing area.
- Must avoid: uniform brown retro templates, travel souvenir albums, forged old archives, decorative seals, and a row of equally sized object cards.
```

#### 暗场信号

```markdown
# Style Brief — Dark-Field Signals
- Visual concept: in a dark field, use localized light sources, trajectories, scales, or rhythmic signals to establish a sense of direction; black is space, not background fill.
- Primary visual focus: one brightly lit object, motion image, glowing trail, or high-contrast glyph form, with other elements building distance and speed around it.
- Poster-level visual move: let a signal line, path, or band of light cross the canvas and connect fact nodes, so the subject and direction of motion remain recognizable from a distance.
- Visual DNA: accent colors serve only nodes, directions, numbers, or warnings; numeric fonts, coordinates, and scanning elements must serve reading rather than create tech noise.
- Information organization: organize route, time, frequency, cohort, or status data along a common coordinate field; keep a stable high-contrast safe zone for body text.
- Must avoid: full-page neon, bloom effects, cyber dashboards, small gray text on black, and packing every data group into a glowing panel.
```

#### 科学观察

```markdown
# Style Brief — Scientific Observation
- Visual concept: treat the page as a verifiable observation field, using objects, sections, relationships, and scale to explain a mechanism, rather than presenting a popular-science column.
- Primary visual focus: a main specimen, an ecological section, a system relationship, or a local zoom; readers see the object first, then understand the mechanism along the annotations.
- Poster-level visual move: let scale, depth, hierarchy, flow direction, or causality run through the whole page, with imagery and legend sharing the same spatial coordinates.
- Visual DNA: colors distinguish objects, relationships, and risks; annotation lines, units, legends, sources, and confidence boundaries stay readable and checkable.
- Information organization: mechanisms, specimens, and evidence are organized through spatial adjacency and connecting relationships; use a small amount of comparison when necessary, and do not default to three-column cards.
- Must avoid: technical-report front pages, white dashboards, equal-width info boxes, decorative data, and unverifiable precise figures.
```

#### 动态拼贴

```markdown
# Style Brief — Dynamic Collage
- Visual concept: use directional glyphs, cropped imagery, color bands, and event fragments to create a manifesto feel, with rhythm coming from content conflicts rather than decorative noise.
- Primary visual focus: one boldly cropped title, figure/object silhouette, or high-contrast image; do not let multiple assets compete evenly for attention.
- Poster-level visual move: interweave the title, imagery, and a diagonal or offset structure to form a direction and posture recognizable from a distance.
- Visual DNA: maintain one dominant contrast relationship; collage edges, numbering, event nodes, and short phrases share a unified logic of angle, weight, and spacing.
- Information organization: compress viewpoints, impacts, events, or stages into a few directional band sequences; body text returns to a stable grid to stay readable.
- Must avoid: promotional ads, price tags, meaningless torn paper, slogan stacking, too many angles, and enlarging all text at once.
```

#### 可拆卸的匿名移动片段

以下仅提供局部构图知识，不提供完整指导；不要为片段命名，也不要将主题与它们一一匹配。首先陈述当前页面的核心信息和主导关系，然后根据需要调整其中一两个动作，或者完全设计自己的动作：

- 让标题穿过、嵌入或穿过承载信息的主要几何图形；事实沿着比例、轴、编号或形状关系占据一席之地——几何图形不仅仅是填充空间的装饰。
- 让对象、纹理、宏或部分跨越画布的主要范围，并将标题直接锚定到可见细节；其他图像形成切片、序列或出处关系，而不是收缩成一排样本卡。
- 让路线、光带或运动轨迹穿过画布并连接事实节点，同时保留必要的地理、场景、分支和时间信息；路径可以由图像、地形或空间关系形成，而不是默认的细线图。
- 让标本、切片、比例、图例、因果关系共享一个连续的观察视野；当沉浸感很重要时，请保持环境的连续性，而不是退化为“左侧主图像+右侧证据框”的技术报告。
- 交织可识别的标题或对象、裁剪图像和方向结构；事件和观点沿着统一的节奏展开，避免让标题的可识别性淹没在拼贴的噪音中。

在实际任务中，使用当前任务特有的概念名称，从主题、资产和阅读关系重新生成 `style.md`。不要先浏览片段寻找“最相似的类别”，也不要在摘要中写“选择某某家庭”。保留一种占主导地位的视觉语法，其他动作仅作为兼容支持；最终方向必须解释为什么它适合当前内容，而不仅仅是交换示例的主题文本和强调色。

## 图像、版权和真实性

- 喜欢主题清晰、可裁剪空间以及与内容真正语义相关的图像。在风格化之前确保对象和证据是正确的。
- 当依靠图像来确定构图时，在决定主题放置、裁剪线和文本安全区域之前实际查看候选图像；不要仅根据关键词或想象力来决定。
- 一旦找到或生成了足够好的候选图像，就让它真正进入主要构图；不要仅仅为了节省实施工作而将特定的人物、工件、材料或环境交换回通用矢量图标、轮廓或示意性替身。
- 当选择摄影或实物图像作为主要视觉语法时，让关键图像在画布上占据足够的面积，以创造令人难忘的印象；不要将所有高质量资源缩小为卡片或装饰性缩略图内的小圆形图像。多幅图像可以通过裁剪、重叠、矩阵、排序或局部缩放形成整体关系。
- 当选择自定义向量作为主要视觉语法时，让轮廓、比例、连接、尺度或注释表达当前主体的具体信息；不要将通用图标、默认流程框或装饰性几何图形冒充为内容。复杂的矢量图同样应该有清晰的视觉焦点和缩略图存储点。
- 混合图像和矢量时，明确分工：图像承载物体证据、材料、人物或环境；矢量携带关系、路径、比例、图例和注释，并将它们在同一组合中相互锚定，而不是分成上下两个不相关的块。
- 使用自有、许可、官方引用或生成的图像。请勿伪造真实的杂志、机构、品牌徽标、期号、水印或归属。
- 人工智能图像不得冒充历史档案、科学证据或真实数据。必要时将它们标记为“人工智能生成的插图”。

## 可编辑的交付和验证

- 尽可能将文本、形状、连接器、数据标签和图例保留为可编辑的 PPTD 元素；不要将整个信息图合并为单个位图。
- 当用户只想要PNG或JPG等图像时，仍然先在PPTD中完成可编辑源，然后将其截图或渲染为图像。
- 使用`kimi-slides check`检查越界、溢出、重叠和结构问题；使用 `kimi-slides screenshot` 检查视觉焦点、阅读顺序、对比度、字体大小和图像裁剪。
- 先看整页缩略图，再看细节：缩略图应揭示主题和至少一个视觉记忆点；如果主要印象是标题、常规卡片和类似按钮的色块，请先重新调整结构，而不是仅微调字体大小和间距。
- 修复问题页面后，再次检查并截图，然后打包PPTX或输出图像。

## 生产前检查表

- 您是否根据主题探索了不同的方向，并在项目中编写了具体的、可操作的`style.md`？
- 画布比例和方向是否符合用户需求、部署场景、内容结构和阅读顺序？
- 核心信息、主要视觉焦点和主导视觉语法是否清晰？
- 在最终确定之前，您是否确实比较了候选英雄图像或结构图，以及所选的资产/人物是否真正进入了主要构图？
- 是否存在可在缩略图大小下识别的海报级视觉动作，而不是普通的内容页面、卡片墙或应用程序界面？
- 地图、时间线、路线、流程图、矩阵和索引是否真正表达了核心关系，而不是充当装饰或密度填充物？
- 颜色、字体、图像处理、构图和图形语言是否来自当前主题，而不是默认重复使用以前的海报案例？
- 事实、数据、日期、地点、单位、标题、来源和版权边界是否完整且可验证？
- 正文、注释和图例在实际输出尺寸下是否可读，而不需要依靠极小的字体来填充信息？
- 屏幕截图中的视觉结果是否真正匹配 `style.md`，而不仅仅是通过语法检查？