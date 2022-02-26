# plugNmeet-recorder

plugNmeet-recorder can be used for recording and RTMP broadcasting. This is a CPU-intensive program. It is recommended
to use a separate server other than the same server you are using for plugNmeet or livekit.. But if you are not using
recording that often then you can use the same server. In that case, it is recommended to use
the [plugnmeet-install](https://github.com/mynaparrot/plugNmeet-install) script for setup.

**Requirements**

1) Nodejs
2) Google Chrome
3) xvfb
4) ffmpeg

**Install dependencies (Ubuntu)**

```
## nodejs
curl -fsSL https://deb.nodesource.com/setup_16.x | sudo -E bash -
sudo apt install -y nodejs

## Google Chrome
curl -sS -o - https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo apt-key add
sudo echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list
sudo apt -y update
sudo apt -y install google-chrome-stable

## xvfb & ffmpeg
sudo apt install -y xvfb ffmpeg

## optional
sudo apt install -y fonts-noto fonts-liberation
```

**Install recorder**

1) To download the latest version, check [release page.](https://github.com/mynaparrot/plugNmeet-recorder/releases)
2) Unzip the recoder.zip file and navigate to the directory from the terminal, then run.

```
cp config_sample.yaml config.yaml
```

4) Change the necessary info inside `config.yaml` file. Redis info should be same as `plugnmeet-server` & `main_path`
   should be same as `plugnmeet-server's` config.yaml `recording_files_path` value. If you're planning to use `NFS` then
   make sure both recorder & plugnmeet server can access that directory. Otherwise, user won't be able to download
   recordings. Also make sure that nodejs has write permission to the path.

5) It's possible to install `plugNmeet-recorder` in multiple server. `plugNmeet-server` will choose appropriate one
   based on availability. In that case change value of `id` inside `config.yaml` file. Make sure that value is unique,
   example: `node_01`, `node_02` ... You can also change the value of `max_limit` based on capacity of the server.

6) Start server `npm start`

**Development**

1) Clone the project & navigate to the directory. Make sure you've nodejs install in your
   PC. https://nodejs.org/en/download/
2) Copy to rename this file:

```
cp config_sample.yaml config.yaml
```

3) Now start server

```
npm install
npm run dev
// to build
npm run build
```
