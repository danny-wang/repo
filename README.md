# 基于golang http包实现的文件服务器
## 代码仓库：https://github.com/danny-wang/repo
## 基本功能

 1. 压缩模式或正常上传
 2. 压缩模式或正常下载
 3. 获取文件服务器状态，包括服务器域名(name:port），当前有多少文件等
 4. 获取某一文件的状态（创建时间，下载路径，超时过期时间，MD5）
 5. 获取某一个文档中的所有文件的状态（可指定是否递归进入子文档，是否只匹配某一个后缀的文件）
 6. 删除过期文件
 7. 备份数据库
## 使用方式
### 方式一：直接使用curl命令调用：
#### 上传文件：
**普通模式上传**：上传文件和服务器中的文件一致
```
curl  -F "file=@bolt" -F dest=/jianwang/bolt.txt  -F expiredTime=2h  -F replaceIfExist=false  "http://localhost:50010/r/upload/"
```
返回值：

```
{"Status":0,"Msg":"file exist","File":{"CreateTime":"0001-01-01T00:00:00Z","Md5":"","ExpiredTime":"0001-01-01T00:00:00Z"}}

```
**压缩模式上传**：对于上传的文件会使用gunzip解压缩，然后存储到文件服务器中。假设被上传的文件是使用gzip压缩后的压缩文件。

```
curl -H "Content-Encoding: gzip"  -F "file=@1.png.gz" -F dest=/jianwang/3.png  -F expiredTime=2h  -F replaceIfExist=false  "http://localhost:50010/r/upload/"
```
返回值：

```
{"Status":0,"Msg":"OK","File":{"CreateTime":"2017-11-22T15:43:08.397174566+08:00","Md5":"e16b119e535c5ebbe8b59ef766335f1c","ExpiredTime":"2017-11-22T17:43:08.397178023+08:00","DownloadPath":"http://localhost:50010/r/download/jianwang/3.png"}}

```

#### 下载文件

```
Normal download: 正常下载，服务器不会压缩数据进行传输
curl -O http://localhost:50010/r/download_file/jianwang/ads.111
Gzip compress mode to download:  指定服务器可以以压缩方式传输文件，客户自己负责解压与否
curl -H "Accept-Encoding: gzip"   http://localhost:50010/r/download_file/jianwang/ads.111 | gunzip >a.dmg
```

**其他请求可以直接阅读repo.go中的注释**
### 方式二：客户端代码调用
**参考repo/client/test.go中的代码**

### 方式三：通过网页
**直接访问 http://localhost:50010/r/list/  ，即可查看数据库中的文件**
## 主要技术

 - 使用boltdb文件数据库存储数据库中文件的元信息
 - 以json格式传输调用的返回值
 - 使用协程定期删除过期的文件
 - 存储文件的MD5码来方便的比较服务器中的文件是否与本地一致
 - 使用glide包管理工具来管理依赖包