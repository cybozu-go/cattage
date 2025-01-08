# Development

1. Prepare a Linux box running Docker.
2. Checkout this repository.

    ```sh
    git clone https://github.com/cybozu-go/cattage
    ```

## Setup CLI tools

1. Install [aqua][].

    <https://aquaproj.github.io/docs/tutorial-basics/quick-start>

2. Install CLI tools.

    ```sh
    cd cybozu-go/cattage
    aqua i -l
    ```

## Development & Debug

1. Launch local Kubernetes cluster.

    ```sh
    cd cybozu-go/cattage
    make dev
    ```

2. Start [Tilt][].

    ```sh
    tilt up
    ```

3. Access: `http://localhost:10350/`
4. Stop the Kubernetes cluster.

    ```sh
    make stop-dev
    ```

[aqua]: https://aquaproj.github.io
[Tilt]: https://tilt.dev
