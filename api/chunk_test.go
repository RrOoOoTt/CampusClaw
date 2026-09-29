package main
import ("strings";"testing")
func TestAutoAndOriginal(t *testing.T){body:=strings.Repeat("这是一段课程资料。",130);orig:=body;chunks,err:=splitText(body,chunkOptions{});if err!=nil||len(chunks)<2||body!=orig{t.Fatal(err,len(chunks))};for _,c:=range chunks{if len([]rune(c.Text))>800||c.Text!=string([]rune(body)[c.Start:c.End]){t.Fatal(c)}};if chunks[1].Start>=chunks[0].End{t.Fatal("overlap missing")}}
func TestHierarchy(t *testing.T){cs,e:=splitText("# 第一章\n内容\n## 第二节\n内容",chunkOptions{Strategy:"hierarchy"});if e!=nil||len(cs)!=2||!strings.HasPrefix(cs[1].Text,"## 第二节"){t.Fatalf("%v %#v",e,cs)}}
func TestCustomProcessedOffsets(t *testing.T){cs,e:=splitText("A  https://example.org  B",chunkOptions{Strategy:"custom",MaxChars:100,Separator:"newline",RemoveURLs:true,CollapseWhitespace:true});if e!=nil||len(cs)!=1||cs[0].Text!="A B"||cs[0].End!=3{t.Fatalf("%v %#v",e,cs)}}
func TestInvalidChunkParameters(t *testing.T){_,e:=splitText("abc",chunkOptions{Strategy:"custom",MaxChars:10,Separator:"newline"});if e==nil{t.Fatal("accepted invalid size")}}
