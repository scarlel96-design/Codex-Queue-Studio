"""Generate Windows PE resources (icon and visual-styles manifest) without external tools."""
from pathlib import Path
import struct

HERE=Path(__file__).resolve().parent
ico=(HERE/'icon.ico').read_bytes()
reserved,typ,count=struct.unpack_from('<HHH',ico,0)
assert reserved==0 and typ==1
pieces=[]
for i in range(count):
 w,h,c,r,planes,bits,size,offset=struct.unpack_from('<BBBBHHII',ico,6+i*16)
 pieces.append((w,h,c,r,planes,bits,ico[offset:offset+size]))
manifest=b'''<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
 <assemblyIdentity name="CodexQueueStudio.Native" version="3.0.0.0" type="win32" processorArchitecture="amd64"/>
 <dependency><dependentAssembly><assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/></dependentAssembly></dependency>
 <trustInfo xmlns="urn:schemas-microsoft-com:asm.v2"><security><requestedPrivileges><requestedExecutionLevel level="asInvoker" uiAccess="false"/></requestedPrivileges></security></trustInfo>
</assembly>'''

group=struct.pack('<HHH',0,1,len(pieces))
for i,(w,h,c,r,planes,bits,img) in enumerate(pieces):
 group+=struct.pack('<BBBBHHIH',w,h,c,r,planes,bits,len(img),i+1)
resources={3:{i+1:img for i,(*_,img) in enumerate(pieces)},14:{1:group},24:{1:manifest}}

section=bytearray();fixups=[]
def alloc(data=b''):
 while len(section)%4:section.append(0)
 pos=len(section);section.extend(data);return pos
def overwrite(pos,data):section[pos:pos+len(data)]=data

def directory(entries):
 pos=alloc(b'\x00'*16);overwrite(pos,struct.pack('<IIHHHH',0,0,0,0,0,len(entries)))
 offsets=[]
 for id in entries:offsets.append((id,alloc(b'\x00'*8)))
 return pos,offsets
root_pos,root_entries=directory(sorted(resources))
for type_id,type_entry in root_entries:
 type_pos,type_entries=directory(sorted(resources[type_id]))
 overwrite(type_entry,struct.pack('<II',type_id,type_pos|0x80000000))
 for id,item in type_entries:
  lang_pos,lang_entries=directory([1033])
  overwrite(item,struct.pack('<II',id,lang_pos|0x80000000))
  leaf=alloc(b'\x00'*16)
  overwrite(lang_entries[0][1],struct.pack('<II',1033,leaf))
  data=resources[type_id][id]
  datapos=alloc(data)
  overwrite(leaf,struct.pack('<IIII',datapos,len(data),0,0))
  fixups.append(leaf)
assert root_pos==0
rawsize=len(section);rawsize=(rawsize+3)&~3;section+=b'\x00'*(rawsize-len(section))
rawptr=60;relptr=rawptr+rawsize;symptr=relptr+10*len(fixups)
# IMAGE_FILE_HEADER
coff=struct.pack('<HHIIIHH',0x8664,1,0,symptr,1,0,4)
# IMAGE_SECTION_HEADER
sec=struct.pack('<8sIIIIIIHHI',b'.rsrc',0,0,rawsize,rawptr,relptr,0,len(fixups),0,0x40000040)
relocs=b''.join(struct.pack('<IIH',offset,0,3) for offset in fixups)
symbol=struct.pack('<8sIhHBB',b'.rsrc',0,1,0,3,0)
syso=coff+sec+section+relocs+symbol+struct.pack('<I',4)
(HERE.parent/'rsrc_windows_amd64.syso').write_bytes(syso)
print('Generated .syso',len(syso),'resources',len(fixups))
