{
  buildGoModule,
  fetchFromGitHub,
}:
buildGoModule rec {
  pname = "sqlc";
  version = "1.31.1";

  src = fetchFromGitHub {
    owner = "sqlc-dev";
    repo = "sqlc";
    rev = "v${version}";
    hash = "sha256-/skb7p3s9TaQE699UCprk1D6S+G/T8Ek9/ADOtS/n44=";
  };

  proxyVendor = true;
  vendorHash = "sha256-+kSAupLQwTzJdgnhlqulEtRcDj9gqSq8uTnWNyDLZew=";

  flags = [ "-trimpath" ];

  ldflags = [
    "-s"
    "-w"
  ];

  subPackages = [ "cmd/sqlc" ];
}
