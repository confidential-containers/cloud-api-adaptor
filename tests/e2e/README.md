# Running end-to-end tests

This directory contain the framework to run complete end-to-end (e2e) tests, i.e., since the build
of software to its installation on provisioned cloud resources and until effectively run tests.

As long as the cloud provider has implemented support on this framework, you can run the tests
as shown below for *libvirt*:

```
$ cd python
$ CLOUD_PROVIDER=libvirt python3 main.py
```

# Adding support for a new provider

In order to add a test pipeline for a new cloud provider, you will need to
create a new class and implement the abstract `base.PipelineABC` methods (decorated with `@abstractmethod`).

Create a new python file (.py) with the name of the provider. For example, suppose that
`awsomeprovider` is the provider's name as you export in the `CLOUD_PROVIDER` variable
in order to build the cloud-api-adaptor binaries, then you **must** create the `awsomeprovider.py` file.

Then you should create a concrete class that implement the abstract methods and properties of the
`base.PipelineABC` class. Following the example above, a snippet of `awsomeprovider.py` should look like below:

```python
from base import PipelineABC

class AwsomeproviderPipeline(PipelineABC):

def get_required_commands(self):
    """
    Implement :func:`base.PipelineABC.get_required_commands`.
    """
    return ['awsomeprovider-cli']

def get_required_envvars(self):
    """
    Implement :func:`base.PipelineABC.get_required_envvars`.
    """
    return ['AWSOMEPROVIDER_AUTH_NAME', 'AWSOMEPROVIDER_AUTH_TOKEN']

@step
def step_foo(self, log):
	"""
    Implement :func:`base.PipelineABC.step_foo`.
    """
    # My implementation of step_foo

(...)
```

>Note: It is important to create the python file and class right names because the pipeline
implementations are loaded on runtime based on standardized names.