---
slug: /1003/get-started/
---

# Get started with Dagger

In this guide, you will learn the basics of Dagger by interacting with a pre-configured environment.
Then you will move on to creating your environment from scratch.

Our pre-configured environment deploys a simple [React](https://reactjs.org/)
application to a unique hosting environment created and managed by us, the Dagger team, for this tutorial.
This will allow you to deploy something "real" right away without configuring your infrastructure first.

In later guides, you will learn how to configure Dagger to deploy to your infrastructure. And, for advanced users,
how to share access to your infrastructure in the same way that we share access to ours now.

## Initial setup

### Install Dagger

First, make sure [you have installed Dagger on your local machine](../1001-install.md).

### Setup example app

You will need a local copy of the [Dagger examples repository](https://github.com/dagger/examples).
NOTE: you may use the same local copy across all tutorials.

```shell
git clone https://github.com/dagger/examples
```

Make sure that all commands are run from the `todoapp` directory:

```shell
cd examples/todoapp
```

### Import the tutorial key

Dagger natively supports encrypted secrets: when a user inputs a value marked as secret
(for example, a password, API token, or ssh key) it is automatically encrypted with that user's key,
and no other user can access that value unless they are explicitly given access.

In the interest of security, Dagger has no way _not_ to encrypt a secret value.
But this causes a dilemma for this tutorial: how do we give unrestricted, public access to our
(carefully sandboxed) infrastructure so that anyone can deploy to it?

To solve this dilemma, we included the private key used to encrypt the tutorial's secret inputs.
Import the key to your Dagger installation, and you're good to go:

```shell
./import-tutorial-key.sh
```

## First deployment

Now that your environment is set up, you are ready to deploy:

```shell
dagger up
```

That's it! You have just made your first deployment with Dagger.

The URL of your newly deployed app should be visible towards the end of the command output.
If you visit that URL, you should see your application live!

## Code, deploy, repeat

This environment is pre-configured to deploy from the `./todoapp` directory,
so you can make any change you want to that directory, then deploy it with `dagger up`.
You can even replace our example React code with any React application!

NOTE: you don't have to commit your changes to the git repository before deploying them.

## Under the hood

This example showed you how to deploy and develop an application that is already configured with Dagger. Now, let's learn a few concepts to help you understand how this was put together.

### The Environment

An Environment holds the entire deployment configuration.

You can list existing environment from the `./todoapp` directory:

```shell
dagger list
```

You should see an environment named `s3`. You can have many environments within your app. For instance, one for `staging`, one for `dev`, etc...

Each environment can have a different kind of deployment code. For example, a `dev` environment can deploy locally; a `staging` environment can deploy to a remote infrastructure, and so on.

### The plan

The plan is the deployment code that includes the logic to deploy the local application to an AWS S3 bucket. From the `todoapp` directory, you can list the code of the plan:

```shell
ls -l ./s3
```

Any code change to the plan will be applied during the next `dagger up`.

### The inputs

The plan can define one or several `inputs`. Inputs may be configuration values, artifacts, or encrypted secrets provided by the user. Here is how to list the current inputs:

```shell
dagger input list
```

The inputs are persisted inside the `.dagger` directory and pushed to your git repository. That's why this example application worked out of the box.

### The outputs

The plan defines one or several `outputs`. They can show helpful information at the end of the deployment. That's how we read the deploy `url` at the end of the deployment. Here is the command to list all outputs:

```shell
dagger output list
```

## What's next?

At this point, you have deployed your first application using Dagger and learned some dagger commands. You are now ready to [learn more about how to program Dagger](/learn/102-dev).