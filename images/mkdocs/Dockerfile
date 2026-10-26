# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM alpine:3.22

RUN apk add --no-cache \
  bash \
  git \
  git-fast-import \
  openssh \
  python3 \
  python3-dev \
  curl \
  build-base \
  && rm -rf /var/cache/apk/*

# Alpine's python is "externally managed" (PEP 668), so pip refuses to install into the system
# site-packages. Install mkdocs and its plugins into a virtualenv instead.
RUN python3 -m venv /opt/venv \
  && /opt/venv/bin/pip install --no-cache-dir --upgrade pip setuptools
ENV PATH="/opt/venv/bin:${PATH}"

COPY requirements.txt /requirements.txt
RUN pip install --no-cache-dir -U -r /requirements.txt

WORKDIR /docs

EXPOSE 3000

COPY entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
