# Linux container coming with go ROOT configured at /go.
FROM golang:1.15

# Add the source code to the container.
ADD . /go/src/bazo-miner

# CD into the source code directory.
WORKDIR /go/src/bazo-miner

# Build the application.
RUN go build -o /bazo-miner

# Define the start command when this container is run.
CMD ["/bazo-miner", "start", "--database", "StoreA.db", "--address", "127.0.0.1:8000", "--bootstrap", "127.0.0.1:8000",  "--wallet", "WalletA.txt", "--commitment", "CommitmentA.txt", "--multisig", "WalletA.txt", "--rootwallet", "WalletA.txt", "--rootcommitment", "CommitmentA.txt", "--root-chparams", "ChParamsA.txt"]