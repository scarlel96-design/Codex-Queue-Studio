"""Generate the Codex Queue Studio multi-resolution app icon."""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter
S=1024
P=Path(__file__).resolve().parent
im=Image.new("RGBA",(S,S))
pix=im.load()
for y in range(S):
 for x in range(S):
  t=(x/S)*.36+(1-y/S)*.23
  pix[x,y]=(int(8+19*t),int(17+37*t),int(44+68*t),255)
mask=Image.new("L",(S,S),0)
ImageDraw.Draw(mask).rounded_rectangle((36,36,988,988),radius=188,fill=255)
im.putalpha(mask)
glow=Image.new("RGBA",im.size)
g=ImageDraw.Draw(glow)
g.rounded_rectangle((320,230,866,715),radius=140,fill=(51,142,252,70))
g.rounded_rectangle((177,349,784,784),radius=125,fill=(96,188,254,100))
g.ellipse((583,581,970,968),fill=(26,207,255,90))
im=Image.alpha_composite(im,glow.filter(ImageFilter.GaussianBlur(45)))
d=ImageDraw.Draw(im)
d.rounded_rectangle((320,230,866,706),radius=139,fill=(41,107,219,230),outline=(123,205,255,230),width=11)
d.rounded_rectangle((178,350,778,786),radius=126,fill=(36,75,150,244),outline=(165,224,255,248),width=14)
d.polygon([(251,757),(397,754),(271,868)],fill=(36,75,150,244))
d.line([(273,851),(251,757)],fill=(165,224,255,248),width=12)
for y,length in [(479,430),(573,330),(668,230)]:
 d.rounded_rectangle((269,y,269+length,y+43),radius=22,fill=(179,215,255,255))
d.ellipse((591,580,979,968),fill=(18,49,111,255),outline=(49,217,255,255),width=39)
d.ellipse((635,623,935,923),fill=(10,35,80,255))
for r in [(779,642,794,693),(779,855,794,905),(659,766,707,781),(867,766,913,781)]:
 d.rounded_rectangle(r,radius=7,fill=(159,232,255,255))
d.line([(786,778),(786,675)],fill=(225,244,255,255),width=28)
d.line([(786,778),(863,829)],fill=(225,244,255,255),width=28)
d.ellipse((766,759,805,798),fill=(234,247,255,255))
im.resize((512,512),Image.Resampling.LANCZOS).save(P/"icon.png",optimize=True)
im.save(P/"icon.ico",format="ICO",sizes=[(16,16),(24,24),(32,32),(48,48),(64,64),(128,128),(256,256)])
print("Generated icon.png and icon.ico")
