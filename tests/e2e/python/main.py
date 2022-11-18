"""
Copyright Confidential Containers Contributors
SPDX-License-Identifier: Apache-2.0
"""
#!/usr/bin/env python3

import importlib
import os
import sys

def main():
    cloud_provider = os.getenv('CLOUD_PROVIDER')
    if cloud_provider is None:
        print("ERROR: 'CLOUD_PROVIDER' variable should be exported")
        return 1

    try:
        module = importlib.import_module(cloud_provider)
        class_name = cloud_provider.capitalize() + 'Pipeline'
        pipeline = getattr(module, class_name)()
    except ModuleNotFoundError:
        print("ERROR: not found '%s' module with the implemeting pipeline." % cloud_provider)
        return 1
    except AttributeError:
        print("ERRO: pipeline class '%s' not found on '%s' module" % (class_name, module.__name__))
        return 1

    return pipeline.run()

if __name__ == "__main__":
    sys.exit(main())
