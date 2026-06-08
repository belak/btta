{
  buildGoModule,
  fetchFromGitHub,
}:
buildGoModule rec {
  pname = "templ";
  version = "0.3.1020";

  src = fetchFromGitHub {
    owner = "a-h";
    repo = "templ";
    rev = "v${version}";
    hash = "sha256-wv7qKZfnavz8lxfaOaIJJySNsXsjke1ADJuv2kgQOHE=";
  };

  vendorHash = "sha256-i4uDGZb3VZUvIyO2Tt53VR1Do/3OYtj6JccGoFnnlbs=";

  subPackages = [ "cmd/templ" ];

  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
    "-extldflags -static"
  ];
}
