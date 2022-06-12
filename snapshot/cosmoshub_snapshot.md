# Getting the 5/20 snapshot of Cosmoshub-4

## Syncing a full node:

From schultzie|Lavender.Five Nodes:

```bash
# update the local package list and install any available upgrades
# # install toolchain and ensure accurate time synchronization
sudo apt-get update -y && sudo apt upgrade -y && sudo apt-get install make build-essential gcc git jq chrony -y
# install go
wget https://golang.org/dl/go1.18.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.18.2.linux-amd64.tar.gz
# source go
cat <<EOF >> ~/.profile
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export GO111MODULE=on
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
EOF
source ~/.profile
go version

# update this before running
git clone https://github.com/cosmos/gaia
cd gaia
git checkout v7.0.0
make install

cd ~/.gaiad/config
wget https://quicksync.io/addrbook.cosmos.json
rm genesis.json
wget https://github.com/cosmos/mainnet/raw/master/genesis.cosmoshub-4.json.gz
gzip -d genesis.cosmoshub-4.json.gz
mv genesis.cosmoshub-4.json ~/.gaia/config/genesis.json
```

default data should be sufficient to get to 5/20: https://quicksync.io/networks/cosmos.html

```bash
sudo cat <<EOF >> /etc/systemd/system/gaiad.service
[Unit]
Description=Gaiad Service
After=network.target

[Service]
Type=simple
User=gaia
WorkingDirectory=/home/gaia
ExecStart=/home/gaia/go/bin/gaiad start
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload && systemctl enable gaiad
sudo systemctl restart gaiad && journalctl -fu gaiad
```

## Exporting state:

From 0xAN|Nodes.Guru:

```bash
gaiad export --height=10562840 --home $HOME/.gaia/ &> cosmos_10562840_genesis.json
cat cosmos_10562840_genesis.json | jq .app_state.auth.accounts[].address | sed 's/"//g' > cosmos_10562840_genesis_addresses.json
```

NOTE: the above sed filter doesn't print the account balances.
