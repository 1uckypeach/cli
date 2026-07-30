# Lark Slides 批量 Lint 报告

工作目录：`tmp/slides_lint/`（XML、Lint JSON、shots/ 截图）

## 汇总

| # | Slides Token | 页数 | errors | warns | 问题页 | 结论 |
|---|---|---|---|---|---|---|
| 01 | NF7ps9bi1la8ArdJCzycaf5Rncb | 13 | 0 | 0 | – | 无问题 |
| 02 | XkVtsIZjOl4f4hdcY65cXM4OnJe | 12 | 0 | 0 | – | 无问题 |
| 03 | F4AIs6Ok5lRMsnd1ps6cZVOlnMd | 16 | 1 | 2 | p3, p5 | 见下 |
| 04 | RQZAskrcil7r89ddRZkcFb1dnoc | 11 | 15 | 0 | p3/p5/p7/p9 | 见下（icon_missing_fill_color）|
| 05 | ZW5UscZRXlvPk9d02uhcEYzan4g | 19 | 0 | 3 | p16 | 见下 |
| 06 | Xytksv7b5lAWtJdaVGrcKkKRnCg | 22 | 0 | 0 | – | 无问题 |
| 07 | AeoYs6ZGrlozGqdJAQ3cW9KnnWd | 17 | 0 | 0 | – | 无问题 |
| 08 | H2FlssNFhleATbd1kJncfL4Lnqg | 16 | 0 | 0 | – | 无问题 |
| 09 | L2qMsLiCylkq5mdmt6fcM1Nunke | 8 | 0 | 0 | – | 无问题 |
| 10 | KqmBsF2Sgl8y1ndfvrFcNxqDnKh | 14 | 0 | 0 | – | 无问题 |
| 11 | NvgJssk2nlVF34d6KcfcECnYnD2 | 16 | 0 | 0 | – | 无问题 |
| 12 | ExsmsZG2cld2LsdUlM5cKSHynkd | 30 | 0 | 0 | – | 无问题 |
| 13 | FuaBsBakKlwpCzdsDMKcdLJYnme | 15 | 10 | 0 | p1, p15 | 见下 |

## 逐页视觉复核

| Deck | 页 | Lint code | 报错内容 | 截图 | 截图观察 | 判定 |
|------|----|-----------|----------|------|----------|------|
| 03 | p3 | bbox_overlap | shape `bBy` 与 `bBn` 边框重叠 | [p3.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/03/F4AIs6Ok5lRMsnd1ps6cZVOlnMd_p003.jpg) | 章节封面页，"01" 大号数字与其下方金色横线 + "第一部分：回顾与复盘" 主标题存在几何 bbox 交叠，视觉上属于设计意图（数字作为背景水印），无阅读干扰 | **误报**（模板堆叠：大号数字水印 + 标题） |
| 03 | p5 | text_may_overflow_shape ×2 | 两个"标杆项目"标题 shape 预估 40px > 26px 可用高度 | [p5.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/03/F4AIs6Ok5lRMsnd1ps6cZVOlnMd_p005.jpg) | 截图显示"标杆项目一" 和 "标杆项目三" 标题实际换行为两行，但均在卡片内部，未穿出卡片或遮挡下方正文 | **误报**（估算保守；文本已在 shape 内换行且未视觉溢出）|
| 04 | p3 | icon_missing_fill_color ×4 | 4 个 `<icon>` 无显式 fillColor | [p3.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/04/RQZAskrcil7r89ddRZkcFb1dnoc_p003.jpg) | 截图右侧 4 个色块（藏青、橙、绿、紫）**都是纯色 `<rect>` 视觉锚点，没有可见图标**；`<icon>` 未填充颜色确实不可见 | **正确**（icon 视觉丢失，需补 fillColor 或改用 rect）|
| 04 | p5 | icon_missing_fill_color ×4 | 同上 | [p5.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/04/RQZAskrcil7r89ddRZkcFb1dnoc_p005.jpg) | 4 个卡片左上角浅色小方块无图标图形，仅剩底色 | **正确**（图标丢失）|
| 04 | p7 | icon_missing_fill_color ×3 | 同上 | [p7.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/04/RQZAskrcil7r89ddRZkcFb1dnoc_p007.jpg) | 3 张场景卡上方灰色大方块本应是场景插图 icon，实际全空 | **正确**（图标丢失）|
| 04 | p9 | icon_missing_fill_color ×4 | 同上 | [p9.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/04/RQZAskrcil7r89ddRZkcFb1dnoc_p009.jpg) | 右侧 4 张收入结构卡片左侧浅色方块本应是分类图标，全部空白 | **正确**（图标丢失）|
| 05 | p16 | text_may_overflow_shape ×3 | 敏感性分析区块 shape `bwQ` 等预估 25px > 20px | [p16.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/05/ZW5UscZRXlvPk9d02uhcEYzan4g_p016.jpg) | 截图右上"敏感性分析"表格及旁注文字均在自己的白色卡片内部，未溢出到相邻元素或页面边界 | **误报**（估算 5px 溢出，实际视觉无干扰）|
| 13 | p1 | image_covers_text ×5 | 背景 `<img> bgK` 覆盖 5 个文本 shape | [p1.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/13/FuaBsBakKlwpCzdsDMKcdLJYnme_p001.jpg) | 截图为纯粹的数据科技蓝紫色抽象背景图，**页面上完全看不到"数据科学实战营+大数据行业导师面对面"等 5 段封面文字** | **正确**（真实漏字：整块封面文案被背景图完全遮盖）|
| 13 | p15 | image_covers_text ×5 | 背景 `<img> bgK` 覆盖 5 个文本 shape | [p15.jpg](file:///Users/bytedance/go/src/github.com/larksuite/cli/tmp/slides_lint/shots/13/FuaBsBakKlwpCzdsDMKcdLJYnme_p015.jpg) | 截图为学校建筑 + 数据科技元素背景图，**"感谢聆听 敬请指导"等结束页文案完全不可见** | **正确**（真实漏字：结束页文案被背景图完全遮盖）|

## 统计

- 检测 13 份 Slides、共 209 页。
- Lint 报出问题的页：**9 页**（跨 4 份 deck）。
- 视觉复核后：
  - **真实缺陷** 4 页 / 25 issues：deck 04 的 4 页 icon 丢失（15）、deck 13 的 p1/p15 封面结束页文字被背景图整块遮盖（10）。
  - **误报** 3 页 / 6 issues：deck 03 p3 模板堆叠、deck 03 p5、deck 05 p16 的 `text_may_overflow_shape` 估算偏保守。

## 复现命令

```bash
# 逐份下载
lark-cli slides +xml-get --as user --presentation <URL> \
  --output tmp/slides_lint/<name>.xml

# 静态 lint
python3 skills/lark-slides/scripts/xml_text_overlap_lint.py \
  --input tmp/slides_lint/<name>.xml > tmp/slides_lint/<name>.json

# 目标页截图
lark-cli slides +screenshot --as user \
  --presentation <TOKEN> --slide-number N \
  --output-dir tmp/slides_lint/shots/<deck>
```
