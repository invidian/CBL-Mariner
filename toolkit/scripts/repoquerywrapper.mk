# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Contains:
#	- Tools to query rpms repos in bulk.

.PHONY: pre-repoquerywrapper clean-repoquerywrapper

repoquerywrapper_state_dir = $(REPO_QUERY_DIR)/repoquerywrapper
repo_urls_file = $(repoquerywrapper_state_dir)/repo_urls.txt
repoquerywrapper_chroot_dir = $(repoquerywrapper_state_dir)/chroot
repoquerywrapper_logs_path = $(LOGS_DIR)/repoquerywrapper/repoquerywrapper.log

$(call create_folder,$(repoquerywrapper_state_dir))

clean-repoquerywrapper: clean-repoquerywrapper
clean: clean-repoquerywrapper
clean-repoquerywrapper:
	@echo Verifying no mountpoints present in $(repoquerywrapper_chroot_dir)
	$(SCRIPTS_DIR)/safeunmount.sh "$(repoquerywrapper_chroot_dir)" && \
	rm -rf $(repoquerywrapper_state_dir)

# We always want to run the repoquerywrapper script, it will decide if it needs to download anything and update the flag file if
# it does, so add the phony target as a dependency to the flag file
.PHONY: repoquerywrapper_always_run_phony
repo-query: $(STATUS_FLAGS_DIR)/repoquerywrapper.flag
$(STATUS_FLAGS_DIR)/repoquerywrapper.flag: $(go-repoquerywrapper) $(chroot_worker) repoquerywrapper_always_run_phony 
	@if [ "$(DISABLE_UPSTREAM_REPOS)" = "y" ]; then \
		echo "ERROR: Upstream repos are disabled (DISABLE_UPSTREAM_REPOS=y), cannot repo-query RPMs"; \
		exit 1; \
	fi
	$(go-repoquerywrapper) \
		--query-input-file $(QUERY_INPUT_FILE) \
		--query-cmd $(QUERY_CMD) \
		--query-output-file $(QUERY_OUTPUT_FILE) \
		$(foreach url,$(PACKAGE_URL_LIST), --repo-url "$(url)") \
		$(foreach repofile,$(REPO_LIST), --repo-file "$(repofile)") \
		--worker-tar $(chroot_worker) \
		--worker-dir $(repoquerywrapper_chroot_dir) \
		--log-file=$(repoquerywrapper_logs_path) \
		--log-level=$(LOG_LEVEL) \
		--cpu-prof-file=$(PROFILE_DIR)/repoquerywrapper.cpu.pprof \
		--mem-prof-file=$(PROFILE_DIR)/repoquerywrapper.mem.pprof \
		--trace-file=$(PROFILE_DIR)/repoquerywrapper.trace \
		$(if $(filter y,$(ENABLE_CPU_PROFILE)),--enable-cpu-prof) \
		$(if $(filter y,$(ENABLE_MEM_PROFILE)),--enable-mem-prof) \
		$(if $(filter y,$(ENABLE_TRACE)),--enable-trace) \
		--timestamp-file=$(TIMESTAMP_DIR)/repoquerywrapper.jsonl