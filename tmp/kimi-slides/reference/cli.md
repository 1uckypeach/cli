# kimi-slides cli

本节列出了 kimi-slides CLI 支持的所有命令。这篇文档是权威参考！ `kimi-slides --help` 返回的指南不正确 - 无需运行该命令。

## 转换

```bash
kimi-slides convert path/deck.pptx 
kimi-slides convert path/deck.pptx -o path/output/ 
```

`-o, --output`：指定输出目录；选修的。如果省略，则会在输入旁边创建一个同名（不带扩展名）的目录：

```text
path/deck.pptx -> path/deck/
```

输出目录结构：

```text
deck/
  deck.pptd
  pages/
    page-1.page
  media/
    <hash>.png
```

> 转换过程中，PPTX 中嵌入的图像将被提取到 `media/` 目录中，并且页面文件中的图像路径将被重写为相对于输出目录的路径：

```yaml
src: ./media/<hash>.png
```

## 检查

```bash
kimi-slides check path/deck 
kimi-slides check path/deck -p 1,3 -s all 
kimi-slides check path/deck -p 2-10 --level keep 
kimi-slides check path/deck --level auto 
```

`check` 的输入必须是包含 `.pptd` 文件的目录。默认情况下，它检查主 `.pptd` 文件和所有页面文件。

参数：

- `-p, --page <spec>`：指定页码，从1开始；默认为所有页面。支持`3`、`1,2`、`2-10`。
- `-s, --severity <spec>`：指定要输出的问题。支持 `all`、`error`、`warning` 以及特定问题类型，例如`MissingField,SrcNotFound`。
- `--level <level>`：处理级别。 `keep` 只检查不修改； `auto` 尝试安全修复。

目前已检查以下问题：
- `YamlParseError`：每个检查的文件必须使用 YAML 解析器正确解析。
- `FileReadError`：对输入目录、`.pptd` 文件、页面文件或资源路径的访问异常。
- `MissingField`：缺少必填字段，例如`.pptd.pages`、`.pptd.size` 或必需的元素字段。
- `InvalidType`：字段类型与其定义不匹配，例如字符串、数字、布尔值、元组、数组、填充、边框等。
- `OutOfRange`：字段值超出可接受范围，例如非正页面大小、非正边界、非法枚举值。
- `InvalidTheme`：引用的主题令牌不存在，例如`$primary` 不存在于 `theme.colors` 中。
- `PageNotFound`：`.pptd.pages` 引用的 `.page` 文件不存在，或者指定的页码超出范围。
- `UnknownField`：发现当前模式无法识别的字段。
- `BoundsOutside`：元素的边界超出页面尺寸。
- `SrcNotFound`：`src`引用的本地资源（图像、图像填充、自定义字体等）不存在。
- `TextOverflow`：文本可能会溢出其文本框。
- `TextUnderFill`：文本可能占据文本框高度的 50% 以下。
- `TextOcclusion`：文本可能会被稍后绘制的元素遮挡。
- `TextDrift`：文本框可以跨越其下方元素的边界。

> 文本相关检查当前使用具有模拟文本尺寸的启发式方法，因此可能存在一些偏差。

`--level auto` 仅执行确定性修复：

- 简单的类型转换，例如`"12"` 为数字，`12` 为字符串，`"true"` 为布尔值。
- 删除无效的可选字段。
- 删除无法安全修复的无效元素。
- 重写修复文件的 YAML 格式。

`check` 以以下格式输出问题：

```text
[whether deterministically fixed][issue type:issue name] file path id="element id" issue details
[fixed: false][Error:InvalidType] pages/page-1.page id="title" Expected 4 numbers for elements[0].bounds
[fixed: false][Warning:TextOverflow] pages/page-2.page id="body" Text may overflow its bounds
```

## 截图

```bash
kimi-slides screenshot path/deck -o path/screenshots/
kimi-slides screenshot path/deck -p 1,3,5 -o path/screenshots/
kimi-slides screenshot path/deck -p 2-6 -o path/screenshots/
```

`screenshot` 的输入必须是包含 `.pptd` 文件的目录。此命令将 `.pptd` 演示文稿渲染为图像以检查页面的视觉结果。

`-o, --output` 是可选的。如果省略，则会在输入旁边创建一个屏幕截图目录：

```text
path/deck/ -> path/deck-screenshots/
```

输出目录结构通常为：

```text
pages/
  page-1.png
  page-2.png
  page-3.png
```

参数：

- `-p, --page <spec>`：指定页码，从1开始；默认为所有页面。支持`3`、`1,2`、`2-10`。
- `-o, --output <path>`：指定截图输出目录。