{name ? "tor-chain/wg-client", tag ? "v1" }:
let 
	system = "x86_64-linux";
	pkgs = import <nixpkgs> {inherit system; };
in with pkgs;
 let
	entrypoint = pkgs.writeTextFile {
		name = "entrypoint.sh";
		executable = true;
		text = ''
#!/bin/sh
VAL="{COPROC[0]}"
ip link add dev wg0 type wireguard
wg setconf wg0 $CONF
ip address add dev wg0 $IP_SRC/24
ip address add dev wg0 $IP_SRC peer $IP_DIST
ip link set up dev wg0
ip route replace default via $GW
iptables -t nat -A POSTROUTING -o wg0 -j MASQUERADE
iptables -I INPUT -p udp --dport 51820 -j LOG --log-prefix 'wireguard iptables: ' --log-level 7
iptables -I OUTPUT -p udp --sport 51820 -j LOG --log-prefix 'wireguard iptables: ' --log-level 7
coproc { exec >&-; read; }; eval exec "$VAL<&-"; wait
		'';
	};

 in
  pkgs.dockerTools.buildImage {
	inherit name tag;
	copyToRoot = pkgs.buildEnv {
		name = "wireguard";
		paths = [
			pkgs.bash
			pkgs.wireguard-tools
			pkgs.iproute2
			pkgs.iptables
		];
	};
	config = {
		WorkingDir = "/config";
		#Tag = "latest";
		Env = [
		];
   		Cmd = [
			entrypoint
		];
		Entrypoint = [ entrypoint ];
 	};
  }
