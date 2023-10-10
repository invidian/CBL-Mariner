Summary:        AsciiDoc is a human readable text document format
Name:           asciidoc
Version:        10.2.0
Release:        1%{?dist}
License:        GPLv2
URL:            https://asciidoc.org/
Group:          System Environment/Development
Vendor:         Microsoft Corporation
Distribution:   Mariner
Source0:        https://github.com/%{name}-py/%{name}-py/releases/download/%{version}/%{name}-%{version}.tar.gz
BuildArch:      noarch
BuildRequires:  python3
BuildRequires:  python3-xml
BuildRequires:  libxslt
BuildRequires:  docbook-style-xsl
BuildRequires:  docbook-dtd-xml
Requires:       python3
Requires:       python3-xml
Requires:       libxslt
Requires:       docbook-style-xsl
Requires:       docbook-dtd-xml

%description
AsciiDoc is a human readable text document format that can be easily converted to other document formats.

%prep
%autosetup -n %{name}-%{version}

%build
autoreconf -v
export CFLAGS="%{optflags}"
./configure  --prefix=%{_prefix} --sysconfdir=%{_sysconfdir}

make %{?_smp_mflags}

%install
rm -rf %{buildroot}%{_infodir}
make DESTDIR=%{buildroot} install

%check
python3 tests/testasciidoc.py update
python3 tests/testasciidoc.py run

%post   -p /sbin/ldconfig
%postun -p /sbin/ldconfig

%files
%defattr(-,root,root)
%license COPYRIGHT
%{_bindir}/*
%{_sysconfdir}/*
%{_mandir}/*

%changelog
* Tue Oct 10 2023 CBL-Mariner Servicing Account <cblmargh@microsoft.com> - 10.2.0-1
- Auto-upgrade to 10.2.0 - Azure Linux 3.0 - package upgrades

*   Wed May 05 2021 Nick Samson <nisamson@microsoft.com> - 9.1.0-1
-   Updated to 9.1.0, removed python2 support, verified license
*   Sat May 09 2020 Nick Samson <nisamson@microsoft.com> - 8.6.10-4
-   Added %%license line automatically
*   Tue May 05 2020 Emre Girgin <mrgirgin@microsoft.com> 8.6.10-3
-   Renaming docbook-xsl to docbook-style-xsl
*   Tue May 05 2020 Emre Girgin <mrgirgin@microsoft.com> 8.6.10-2
-   Renaming docbook-xml to docbook-dtd-xml
*   Wed Mar 25 2020 Emre Girgin <mrgirgin@microsoft.com> 8.6.10-1
-   Update to verison 8.6.10 and python3 implementation. License verified.
*   Tue Sep 03 2019 Mateusz Malisz <mamalisz@microsoft.com> 8.6.9-5
-   Initial CBL-Mariner import from Photon (license: Apache2).
*   Wed Sep 06 2017 Anish Swaminathan <anishs@vmware.com> 8.6.9-4
-   Use system sysconfdir
*   Tue Apr 25 2017 Priyesh Padmavilasom <ppadmavilasom@vmware.com> 8.6.9-3
-   Fix arch
*   Tue May 24 2016 Priyesh Padmavilasom <ppadmavilasom@vmware.com> 8.6.9-2
-   GA - Bump release of all rpms
*   Fri Jun 5 2015 Touseef Liaqat <tliaqat@vmware.com> 8.6.9-1
-   Initial build.  First version
