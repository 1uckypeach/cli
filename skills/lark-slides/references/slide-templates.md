# Slide XML Template

The slide XML template can be copied directly for use. Plain text/shape templates can be wrapped using `jq` and passed to `xml_presentation.slide.create`:

```bash
lark-cli slides xml_presentation.slide create --as user \
  --params '{"xml_presentation_id":"YOUR_ID"}' \
  --data "$(jq -n --arg content 'PASTE_XML_HERE' '{slide:{content:$content}}')"
```

> **Do not submit templates with pictures directly according to the above command. ** When creating a new PPT, you can use `src="@./local.png"` in `+create --slides`, and the CLI will automatically upload and replace it with `file_token`; when adding or modifying images to an existing PPT, you must first use `slides +media-upload` to get the `file_token`, and then write it into `<img src="...">`.

## Dark cover page

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)"/></fill></style>
  <data>
    <shape type="text" topLeftX="80" topLeftY="160" width="800" height="70">
<content><p textAlign="center"><strong><span color="rgb(255,255,255)" fontSize="44">Main title</span></strong></p></content>
    </shape>
    <shape type="text" topLeftX="80" topLeftY="250" width="800" height="35">
<content><p textAlign="center"><span color="rgb(148,163,184)" fontSize="20">Subtitle</span></p></content>
    </shape>
    <shape type="text" topLeftX="80" topLeftY="420" width="800" height="25">
<content><p textAlign="center"><span color="rgb(100,116,139)" fontSize="14">Bottom information</span></p></content>
    </shape>
  </data>
</slide>
```

## Light content page

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="rgb(248,250,252)"/></fill></style>
  <data>
    <shape type="rect" topLeftX="60" topLeftY="40" width="4" height="35">
      <fill><fillColor color="rgb(59,130,246)"/></fill>
    </shape>
    <shape type="text" topLeftX="76" topLeftY="36" width="600" height="45">
<content><p><strong><span color="rgb(15,23,42)" fontSize="28">Page title</span></strong></p></content>
    </shape>
    <shape type="text" topLeftX="60" topLeftY="100" width="840" height="380">
      <content textType="body" lineSpacing="multiple:1.8">
<p><span color="rgb(51,65,85)" fontSize="15">Text paragraph</span></p>
        <ul>
<li><p><span color="rgb(51,65,85)" fontSize="15">Point 1</span></p></li>
<li><p><span color="rgb(51,65,85)" fontSize="15">Point 2</span></p></li>
<li><p><span color="rgb(51,65,85)" fontSize="15">Point 3</span></p></li>
        </ul>
      </content>
    </shape>
  </data>
</slide>
```

## Data card page (horizontal indicators)

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="rgb(248,250,252)"/></fill></style>
  <data>
    <shape type="text" topLeftX="60" topLeftY="36" width="600" height="45">
<content><p><strong><span color="rgb(15,23,42)" fontSize="28">Data Overview</span></strong></p></content>
    </shape>
<!-- Card 1 -->
    <shape type="rect" topLeftX="60" topLeftY="100" width="260" height="140">
      <fill><fillColor color="rgb(255,255,255)"/></fill>
      <border color="rgba(0,0,0,0.08)" width="1"/>
    </shape>
    <shape type="text" topLeftX="60" topLeftY="115" width="260" height="50">
<content><p textAlign="center"><strong><span color="rgb(59,130,246)" fontSize="36">Value</span></strong></p></content>
    </shape>
    <shape type="text" topLeftX="60" topLeftY="175" width="260" height="25">
<content><p textAlign="center"><span color="rgb(100,116,139)" fontSize="14">Indicator name</span></p></content>
    </shape>
<!-- Card 2: topLeftX="350" -->
<!-- Card 3: topLeftX="640" -->
  </data>
</slide>
```

## Format with pictures

> **Key reminder**: `<img>` will not be cropped if its `width:height` = the proportion of the original image. Each template is marked with the proportion of the picture frame and the recommended ratio of the original picture. **Check the proportions of your material before choosing a template**, and don't force it (if you put a horizontal picture into a vertical frame, most of it will be cropped off left and right). Replace `@./your-image.jpg` with the actual path (only `+create --slides` supports the `@` placeholder; in other scenarios, you need to use `slides +media-upload` to get `file_token` first).

### Right picture on the cover (left picture on the right)

Picture frame 400×225 (**16:9**), recommended original picture: banner 16:9 (desktop wallpaper, product banner, landscape photo)

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)"/></fill></style>
  <data>
    <shape type="text" topLeftX="60" topLeftY="180" width="450" height="80">
<content><p><strong><span color="rgb(255,255,255)" fontSize="44">Main title</span></strong></p></content>
    </shape>
    <shape type="text" topLeftX="60" topLeftY="270" width="450" height="40">
<content><p><span color="rgb(186,230,253)" fontSize="20">Subtitle</span></p></content>
    </shape>
    <line startX="60" startY="350" endX="180" endY="350">
      <border color="rgb(59,130,246)" width="3"/>
    </line>
    <shape type="text" topLeftX="60" topLeftY="370" width="450" height="30">
<content><p><span color="rgb(203,213,225)" fontSize="13">Bottom information</span></p></content>
    </shape>
<!-- Picture frame 400×225 = 16:9; original picture recommends 16:9 banner -->
    <img src="@./your-landscape.jpg" topLeftX="540" topLeftY="157" width="400" height="225"/>
  </data>
</slide>
```

### Three cards with pictures (above and below)

Each picture frame is 240×180 (**4:3**). It is recommended that the original picture be: 4:3 or close to a square picture (product photos, screenshots, icons)

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="rgb(248,250,252)"/></fill></style>
  <data>
    <shape type="text" topLeftX="60" topLeftY="40" width="600" height="45">
<content><p><strong><span color="rgb(15,23,42)" fontSize="28">Core Highlights</span></strong></p></content>
    </shape>
    <line startX="60" startY="95" endX="140" endY="95">
      <border color="rgb(59,130,246)" width="3"/>
    </line>

<!-- Card 1 -->
    <shape type="rect" topLeftX="60" topLeftY="130" width="270" height="360">
      <fill><fillColor color="rgb(255,255,255)"/></fill>
      <border color="rgba(0,0,0,0.08)" width="1"/>
    </shape>
<!-- Picture frame 240×180 = 4:3; original picture recommended 4:3 -->
    <img src="@./your-image-1.jpg" topLeftX="75" topLeftY="150" width="240" height="180"/>
    <shape type="text" topLeftX="75" topLeftY="345" width="240" height="30">
<content><p><strong><span color="rgb(15,23,42)" fontSize="18">Feature 1</span></strong></p></content>
    </shape>
    <shape type="text" topLeftX="75" topLeftY="380" width="240" height="90">
<content><p><span color="rgb(71,85,105)" fontSize="14">Short description copy, controlled within two lines. </span></p></content>
    </shape>

<!-- Card 2: Copy card 1, change topLeftX of shape/img to 345 / 360 -->
<!-- Card 3: Copy card 1, change topLeftX of shape/img to 630 / 645 -->
  </data>
</slide>
```

### Left and right columns (picture on the left, text on the right)

Picture frame 360×540 (**2:3 vertical**), it is recommended that the original image: 2:3 or 3:4 vertical (portraits, product vertical shots, posters)

> If you only have a banner image, don’t shoehorn it into this vertical frame – use a “top banner + text below” layout instead (change the frame here to a 960×240 horizontal bar at the top).

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="rgb(255,255,255)"/></fill></style>
  <data>
<!-- Picture frame 360×540 = 2:3; the original picture is recommended to be 2:3 or 3:4 vertical -->
    <img src="@./your-portrait.jpg" topLeftX="0" topLeftY="0" width="360" height="540"/>

    <shape type="text" topLeftX="410" topLeftY="80" width="490" height="50">
<content><p><strong><span color="rgb(15,23,42)" fontSize="30">Scene title</span></strong></p></content>
    </shape>
    <line startX="410" startY="140" endX="490" endY="140">
      <border color="rgb(59,130,246)" width="3"/>
    </line>
    <shape type="text" topLeftX="410" topLeftY="160" width="490" height="50">
<content><p><span color="rgb(71,85,105)" fontSize="16">Describe the value of this scene in one sentence. </span></p></content>
    </shape>
    <shape type="text" topLeftX="410" topLeftY="230" width="490" height="250">
      <content textType="body" lineSpacing="multiple:1.8">
        <ul>
<li><p><span color="rgb(51,65,85)" fontSize="15">Point 1</span></p></li>
<li><p><span color="rgb(51,65,85)" fontSize="15">Point 2</span></p></li>
<li><p><span color="rgb(51,65,85)" fontSize="15">Point 3</span></p></li>
        </ul>
      </content>
    </shape>
  </data>
</slide>
```

## Dark end page

```xml
<slide xmlns="http://www.larkoffice.com/sml/2.0">
  <style><fill><fillColor color="linear-gradient(135deg,rgba(15,23,42,1) 0%,rgba(56,97,140,1) 100%)"/></fill></style>
  <data>
    <shape type="text" topLeftX="80" topLeftY="190" width="800" height="55">
<content><p textAlign="center"><strong><span color="rgb(255,255,255)" fontSize="36">Thank you or call to action</span></strong></p></content>
    </shape>
    <line startX="410" startY="260" endX="550" endY="260">
      <border color="rgb(59,130,246)" width="2"/>
    </line>
    <shape type="text" topLeftX="80" topLeftY="280" width="800" height="30">
<content><p textAlign="center"><span color="rgb(148,163,184)" fontSize="16">Supplementary instructions</span></p></content>
    </shape>
  </data>
</slide>
```
