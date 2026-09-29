package main

import (
 "errors"
 "math"
 "regexp"
 "strings"
)

type chunkOptions struct {
 Strategy string `json:"strategy"`
 MaxChars int `json:"max_chars"`
 OverlapRatio float64 `json:"overlap_ratio"`
 Separator string `json:"separator"`
 RemoveURLs bool `json:"remove_urls"`
 RemoveEmails bool `json:"remove_emails"`
 CollapseWhitespace bool `json:"collapse_whitespace"`
}
type chunk struct { Text string; Index,Start,End int }
var urlPattern=regexp.MustCompile(`https?://[^\s]+`)
var emailPattern=regexp.MustCompile(`[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}`)
var spacesPattern=regexp.MustCompile(`\s+`)
func validateOptions(o chunkOptions)(chunkOptions,error){
 if o.Strategy=="" {o.Strategy="auto"}
 switch o.Strategy {
 case "auto","hierarchy": o.MaxChars=800;o.OverlapRatio=.10;o.Separator="";o.RemoveURLs=false;o.RemoveEmails=false;o.CollapseWhitespace=false
 case "custom":
  if o.MaxChars<100||o.MaxChars>2000||math.IsNaN(o.OverlapRatio)||math.IsInf(o.OverlapRatio,0)||o.OverlapRatio<0||o.OverlapRatio>.5 {return o,errors.New("invalid chunk parameters")}
  if o.Separator!="newline"&&o.Separator!="blankline"&&o.Separator!="period" {return o,errors.New("invalid separator")}
 default:return o,errors.New("invalid strategy")
 }
 return o,nil
}
func splitText(original string, opt chunkOptions)([]chunk,error){
 o,err:=validateOptions(opt);if err!=nil{return nil,err}
 text:=original
 if o.Strategy=="custom" {if o.RemoveURLs{text=urlPattern.ReplaceAllString(text,"")};if o.RemoveEmails{text=emailPattern.ReplaceAllString(text,"")};if o.CollapseWhitespace{text=spacesPattern.ReplaceAllString(text," ")}}
 runes:=[]rune(text);if len(runes)==0{return nil,nil}
 result:=[]chunk{}
 add:=func(start,end int){if start<end {result=append(result,chunk{Text:string(runes[start:end]),Index:len(result),Start:start,End:end})}}
 if o.Strategy=="hierarchy" {
  starts:=[]int{0}
  for i:=0;i<len(runes);i++ {if (i==0||runes[i-1]=='\n')&&runes[i]=='#' {j:=i;for j<len(runes)&&runes[j]=='#'&&j-i<4 {j++};if j-i>=1&&j-i<=3&&j<len(runes)&&runes[j]==' '&&i>starts[len(starts)-1] {starts=append(starts,i)}}}
  for i,start:=range starts {end:=len(runes);if i+1<len(starts){end=starts[i+1]};windowChunks(runes,start,end,800,80,"auto",add)}
 } else {overlap:=80;if o.Strategy=="custom"{overlap=int(float64(o.MaxChars)*o.OverlapRatio)};windowChunks(runes,0,len(runes),o.MaxChars,overlap,o.Strategy+":"+o.Separator,add)}
 return result,nil
}
func windowChunks(s []rune,start,end,max,overlap int,mode string,add func(int,int)){
 for pos:=start;pos<end;{
  stop:=pos+max;if stop>=end {add(pos,end);return}
  min:=pos+max/2;best:=0
  for i:=min;i<stop;i++ {match:=false
   switch mode {
   case "auto":match=s[i]=='\n'||s[i]=='。'||s[i]=='.'||s[i]=='！'||s[i]=='？'||s[i]=='!'||s[i]=='?'
   case "custom:newline":match=s[i]=='\n'
   case "custom:blankline":match=s[i]=='\n'&&i>0&&s[i-1]=='\n'
   case "custom:period":match=s[i]=='。'||s[i]=='.'
   }
   if match {best=i+1}
  }
  if best>pos+overlap {stop=best};add(pos,stop)
  next:=stop-overlap;if next<=pos {next=stop};pos=next
 }
}
func excerpt(s string)string {r:=[]rune(strings.TrimSpace(s));if len(r)>240 {return string(r[:240])+"…"};return string(r)}
