#pkgs ? import <nixpkgs> { system = "x86_64-linux"};
#with import <nixpkgs> {}; 
{name ? "tor-chain/ovpn-client", tag ? "v1" }:
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
ip route replace default via $GW
iptables -t nat -A POSTROUTING -o tun0 -j MASQUERADE
mkdir /dev/net
mknod /dev/net/tun c 10 200
chmod 600 /dev/net/tun
exec openvpn --config $CONF
		'';
	};

 in
  pkgs.dockerTools.buildImage {
	inherit name tag;
	contents = [
		pkgs.bash
		pkgs.openvpn
		pkgs.coreutils
		pkgs.iproute2
		pkgs.iptables
	];
	config = {
		WorkingDir = "/config";
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
