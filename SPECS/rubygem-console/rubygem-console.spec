%global debug_package %{nil}
%global gem_name console
Summary:        Logging for Ruby
Name:           rubygem-console
Version:        1.23.2
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Development/Languages
URL:            https://socketry.github.io/console/
Source0:        https://github.com/socketry/console/archive/refs/tags/v%{version}.tar.gz#/%{gem_name}-%{version}.tar.gz
BuildRequires:  ruby
Requires:       rubygem-fiber-local
Provides:       rubygem(%{gem_name}) = %{version}-%{release}

%description
Provides console logging for Ruby applications.
Implements fast, buffered log output.

%prep
%setup -q -n %{gem_name}-%{version}

%build
gem build %{gem_name}

%install
gem install -V --local --force --install-dir %{buildroot}/%{gemdir} %{gem_name}-%{version}.gem

%files
%defattr(-,root,root,-)
%{gemdir}

%changelog
* Wed Oct 11 2023 CBL-Mariner Servicing Account <cblmargh@microsoft.com> - 1.23.2-1
- Auto-upgrade to 1.23.2 - Azure Linux 3.0 - package upgrades

* Tue Jul 19 2022 Neha Agarwal <nehaagarwal@microsoft.com> - 1.10.1-3
- Add provides.

* Tue Mar 22 2022 Neha Agarwal <nehaagarwal@microsoft.com> - 1.10.1-2
- Build from .tar.gz source.

* Wed Jan 06 2021 Henry Li <lihl@microsoft.com> - 1.10.1-1
- License verified
- Original version for CBL-Mariner
