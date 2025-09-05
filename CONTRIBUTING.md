# Contributing to Myncer

First off, thank you for considering contributing to Myncer! We're excited to have you on board. This guide will help you get your development environment set up and running smoothly.

## Prerequisites

Before you begin, ensure you have the following installed on your system:

-   [Git](https://git-scm.com/)
-   [Nix Package Manager](https://nixos.org/download.html)

### For Windows Users

It is **highly recommended** to use [WSL2 (Windows Subsystem for Linux)](https://learn.microsoft.com/en-us/windows/wsl/install). The development environment is based on Nix, which works best in a Linux environment. All instructions below assume you are working within a WSL2 terminal.

## Setup Instructions

Follow these steps to get your development environment ready.

### 1. Clone the Repository

```shell
git clone https://github.com/hansbala/myncer.git
cd myncer
```

### 2. Configure Your WSL2 Environment (Important for Windows Users)

These steps will prevent common issues related to memory limits and locale warnings.

#### Increase WSL2 Memory

By default, WSL2 has a low memory limit which can cause processes (especially during code generation) to fail.

1.  Open PowerShell (not your WSL terminal) and shut down WSL completely:
    ```powershell
    wsl --shutdown
    ```2.  In Windows, navigate to your user profile folder (you can type `%userprofile%` in the File Explorer address bar).
3.  Create a new file named `.wslconfig` (ensure it has no `.txt` extension).
4.  Add the following content to the file to increase the available memory. Adjust `8GB` to a reasonable amount for your system (e.g., half of your total RAM).

    ```ini
    [wsl2]
    memory=8GB
    ```
5.  Save the file. The new settings will apply the next time you start your WSL distribution.

#### Fix Locale Warnings

You might see `setlocale` warnings in your terminal. To fix them, run the following commands inside your WSL terminal:

```shell
sudo locale-gen en_US.UTF-8
sudo update-locale
```
After running these, close and reopen your WSL terminal.

### 3. Install Dependencies

The project uses Nix Flakes to manage all dependencies (`go`, `nodejs`, `buf`, etc.). The process is a two-step to handle both system-level and project-level dependencies.

1.  **Install Node.js Dependencies:**
    The frontend contains necessary code generation plugins. Navigate to the web app's directory and install them using `pnpm`.

    ```shell
    cd myncer-web
    pnpm install
    cd ..
    ```

2.  **Activate the Nix Shell:**
    Now, from the root of the project, activate the Nix development environment. The first time you run this, it will download all required tools and run the `shellHook` defined in `flake.nix` to install Go plugins.

    ```shell
    nix develop
    ```
    Once it's finished, you'll see the welcome message `🧪 Myncer flake shell ready`, and your terminal will be inside the Nix shell with all tools available in your `PATH`.

### 4. Configure Application Secrets

The application requires API keys and secrets to run. Follow the instructions in the main `README.md` to create your configuration files:
- Create a `server/config.dev.textpb` file for the backend.
- Create a `.env` file in the project root for Docker Compose and the frontend.

Fill in the necessary credentials for Spotify, YouTube, etc.

## Running the Application

All commands should be run from within the Nix shell (`nix develop`).

-   **Run the full stack (Web, Server, DB) with Docker:**
    ```shell
    make up
    ```
-   **Stop and remove all containers:**
    ```shell
    make down
    ```
-   **Run only the server for development:**
    ```shell
    make server-dev
    ```
-   **Run only the web app for development:**
    ```shell
    make web-dev
    ```
-   **Start just the database:**
    ```shell
    make db-up
    ```

## Common Development Tasks

### Generating Protobuf Code

If you make any changes to the `.proto` files in the `proto` directory, you need to regenerate the Go and TypeScript code.

```shell
# Make sure you are inside the nix develop shell
make proto
```

This command runs `buf generate`, which uses the configuration in `buf.gen.yaml` to create the necessary files in the `server/proto` and `myncer-web/src/generated_grpc` directories.

## Troubleshooting

Here are some common issues you might encounter and how to solve them.

-   **Error:** `nix: command not found`
    -   **Cause:** You have just installed Nix, and your current terminal session hasn't loaded the new configuration.
    -   **Solution:** **Close your WSL terminal window completely and open a new one.**

-   **Error:** `plugin protoc-gen-es: signal: killed`
    -   **Cause:** WSL ran out of memory while executing the code generation plugin.
    -   **Solution:** Follow the instructions in **Step 2: Configure Your WSL2 Environment** to increase WSL's memory limit using a `.wslconfig` file.

-   **Error:** `bash: $'\r': command not found` or `: invalid version: version "latest\r" invalid`
    -   **Cause:** The file (`flake.nix` or another script) was saved with Windows-style line endings (`CRLF`) instead of Unix-style (`LF`).
    -   **Solution:** Install `dos2unix` and convert the problematic file.
        ```shell
        sudo apt-get update && sudo apt-get install dos2unix
        dos2unix flake.nix
        ```

-   **Error:** `Failure: plugin ... not found in $PATH`
    -   **Cause:** The required code generation plugins are not available in the environment's `PATH`. This usually happens if dependencies were not installed correctly or in the right order.
    -   **Solution:**
        1. Make sure you have run `pnpm install` inside the `myncer-web` directory.
        2. Exit the Nix shell (`exit`) and re-enter it (`nix develop`) to ensure the `shellHook` runs correctly and installs all Go plugins.

---
