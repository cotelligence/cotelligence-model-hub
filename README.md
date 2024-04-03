# Cotelligence Model Hub

Cotelligence Model Hub is a part of the Cotelligence Project. It is designed to manage various aspects of the project
including pods, API proxy, auto-scaling, and models.

## Features

- **Pod Management**: Efficient handling and organization of pods.
- **API Proxy**: Proxy model inference calls to the actual api providers.
- **Auto-Scaling**: Dynamically scales resources based on workload.
- **Model Management**: Handles the lifecycle of different models in the project.

## Pre-commit Hook Installation

Pre-commit hooks are scripts that run automatically before each commit to check your code and ensure its quality.

```bash
brew install pre-commit
pre-commit install
go install golang.org/x/tools/cmd/goimports@latest
```

## Usage

- Install the required dependencies:

```bash
go mod download
```

- Fill in your .env file with the required environment variables

- Run the application:

```bash
godotenv -f .env go run main.go
```

## Contributing

If you want to contribute to this project, please create a new branch and submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).

## Contact

If you have any questions, feel free to reach out to us at [it@cotelligence.io](mailto:it@cotelligence.io).
