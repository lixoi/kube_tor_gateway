#pkgs ? import <nixpkgs> { system = "x86_64-linux"};
#with import <nixpkgs> {}; 
{name ? "tor-chain/wg-client", tag ? "latest" }:
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
coproc { exec >&-; read; }; eval exec "$VAL<&-"; wait
		'';
	};

 in
  pkgs.dockerTools.buildImage {
	inherit name tag;
	contents = [
		#pkgs.busybox
		pkgs.bash
		pkgs.wireguard 
		pkgs.iproute2
		pkgs.iptables
		#entrypoint
	];

	#runAsRoot = ''
		#!${pkgs.runtimeShell}		
		#iptables -t nat -A POSTROUTING -o wg0 -j MASQUERADE
	#'';
	#extraCommands = ''
	#	mkdir /mydir/my
	#'';
	config = {
		WorkingDir = "/config";
		#Tag = "latest";
		Env = [
		];
   		Cmd = [
			entrypoint
			#"${pkgs.bash}/bin/sh" "/configure/init.sh"
			#"${pkgs.bash}/bin/sh" "-c" "echo hello > foo; ls -l"
		];
		Entrypoint = [ entrypoint ];
 	};
  }
