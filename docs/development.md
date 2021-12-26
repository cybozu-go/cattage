# Development

Cattage can be developed in Tilt.

1. Prepare a Linux box running Docker.
2. Checkout this repository. 

    ```console
    $ git clone https://github.com/cybozu-go/cattage
    ```

3. Launch local Kubernetes cluster.

    ```console
    $ cd cattage
    $ make dev
    ```

4. Start Tilt.

    ```console
    $ make tilt
    $ ./bin/tilt up
    ```

5. Access: http://localhost:10350/
6. Stop the Kubernetes cluster.

    ```console
    $ make stop-dev
    ```

[Tilt]: https://tilt.dev
