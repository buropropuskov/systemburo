// Shared card contracts keep each source's original row mapper and refresh flow together.
// Registry IDs remain distinct from the active application entity IDs.

// Existing readers replace row objects only after accepting a successful GET.
// A swallowed error or superseded request leaves the old row in place; it must
// not overwrite a period already confirmed by PUT. A new row is authoritative,
// including when it contains a later change made by another operator.
function refreshedPeriodRow(rows, id, previous) {
  const row = rows.find(item => item.id === id);
  return row && row !== previous ? row : null;
}

export const carsTablePeriodDetails = {
  computed: {
    entityDetailsProps() {
      return {
        show: this.showVehicleDetails, vehicle: this.selectedVehicle,
        allUnloadingPlaces: this.allUnloadingPlaces, allTables: this.allTables,
        licensePlateFormats: this.licensePlateFormats,
        currentUserId: this.currentUserId, currentUserName: this.currentUserName,
        showCarFeatures: true, source: 'carstable', periodTableId: this.tableId,
      };
    },
  },
  methods: {
    async onManualEntityAttached(result) {
      const id = this.selectedVehicle?.id;
      if (!id || result.entity_id !== id || result.entity_kind !== 'car') return;
      // The old manual context no longer exists for this row; do not keep it
      // editable while the table accepts its authoritative new binding.
      this.showVehicleDetails = false;
      await this.loadData();
    },
    async onEntityPeriodChanged(result) {
      const id = this.selectedVehicle?.id;
      const tableId = this.tableId;
      if (!id || result.entity_id !== id) return;
      const previous = this.itemsData.find(item => item.id === id);
      this.selectedVehicle = { ...this.selectedVehicle, ...result.effective_period, period_mode: result.period_mode };
      await this.loadData();
      if (this.tableId !== tableId || !this.showVehicleDetails || this.selectedVehicle?.id !== id) return;
      const fresh = refreshedPeriodRow(this.itemsData, id, previous);
      if (fresh) this.openVehicleDetails(fresh);
    },

    openVehicleDetails(item) {
      this.selectedVehicle = {
        ...item,
        plateNumber: item.car_number,
        mark: item.car_brand,
        formatId: null,
        organization: item.organization_name,
        organizationId: item.organization_id,
        company: item.company,
        companyId: item.company_id,
        isExisting: true,
        unloadPlaces: item.unload_place_ids || [],
        entry_date_to: item.entry_date_to,
        entry_time_from: item.entry_time_from,
        entry_time_to: item.entry_time_to,
        applicationId: item.applicationId,
        entry_checked: item.entry_checked,
        exit_checked: item.exit_checked
      };
      this.showVehicleDetails = true;
    },
  },
};

export const peopleTablePeriodDetails = {
  computed: {
    entityDetailsProps() {
      return {
        show: this.showDetailsModal, employee: this.selectedEmployee,
        allTables: this.allTables, currentUserId: this.currentUserId,
        currentUserName: this.currentUserName, source: 'peopletable',
        periodTableId: this.currentTableId,
      };
    },
  },
  methods: {
    async onManualEntityAttached(result) {
      const id = this.selectedEmployee?.id;
      if (!id || result.entity_id !== id || result.entity_kind !== 'employee') return;
      this.showDetailsModal = false;
      await this.loadData();
    },
    async onEntityPeriodChanged(result) {
      const id = this.selectedEmployee?.id;
      const tableId = this.currentTableId;
      if (!id || result.entity_id !== id) return;
      const previous = this.itemsData.find(item => item.id === id);
      const period = result.effective_period;
      this.selectedEmployee = { ...this.selectedEmployee, ...period, period_mode: result.period_mode,
        pass_time: [period.entry_time_from?.slice(0, 5), period.entry_time_to?.slice(0, 5)].filter(Boolean).join(' - ') };
      await this.loadData();
      if (this.currentTableId !== tableId || !this.showDetailsModal || this.selectedEmployee?.id !== id) return;
      const fresh = refreshedPeriodRow(this.itemsData, id, previous);
      if (fresh) this.openEmployeeDetails(fresh);
    },

    openEmployeeDetails(item) {
      this.selectedEmployee = {
        id: item.id,
        last_name: item.last_name,
        first_name: item.first_name,
        middle_name: item.middle_name,
        position: item.position,
        citizenshipName: item.citizenshipName,
        passport_series_number: item.passport_series_number,
        patent_number: item.patent_number,
        other_permission: item.other_permission,
        organization: item.organization_name,
        organizationId: item.organization_id,
        company: item.company,
        companyId: item.company_id,
        entry_date_to: item.entry_date_to,
        pass_time: item.pass_time,
        target_tables: item.target_tables || [],
        territory_status: item.territory_status,
        applicationId: item.applicationId
      };
      this.showDetailsModal = true;
    },
  },
};

export const carsRegistryPeriodDetails = {
  computed: {
    entityDetailsProps() {
      return {
        show: this.showDetailsViewModal, vehicle: this.detailsCar,
        allUnloadingPlaces: this.allUnloadingPlaces, licensePlateFormats: [],
        currentUserId: this.ownershipInfo?.user_id || null, currentUserName: '',
        showCarFeatures: false, source: 'carsview',
      };
    },
  },
  methods: {
    async onEntityPeriodChanged(result) {
        const id = this.detailsCar?.id;
        if (!id || result.entity_id !== this.detailsCar.activeCarId) return;
        const previous = this.carsData.find(car => car.id === id);
        this.detailsCar = { ...this.detailsCar, ...result.effective_period, period_mode: result.period_mode };
        await this.fetchCars();
        if (!this.showDetailsViewModal || this.detailsCar?.id !== id) return;
        const fresh = refreshedPeriodRow(this.carsData, id, previous);
        if (fresh) this.openCarDetails(fresh);
    },

    openCarDetails(car) {
        this.detailsCar = {
            id: car.id,
            plateNumber: car.number,
            roof_access: car.roof_access,
            free_parking: car.free_parking,
            individual_roof_access: car.individual_roof_access,
            individual_free_parking: car.individual_free_parking,
            mark: car.mark,
            formatId: car.format_id || null,
            organization: car.active_app_org_name || car.organization_name || null,
            organizationId: car.organization_id || null,
            company: car.active_app_company_name || car.company_name || null,
            companyId: car.company_id || null,
            isExisting: true,
            // active_car_id - id заявочной строки активной заявки; по нему тянем
            // места разгрузки и статус территории (в реестре их нет).
            activeCarId: car.active_car_id || null,
            // id самой заявки - для кнопки "Открыть заявку" (open-application).
            applicationId: car.active_application_id || null,
            unloadPlaces: this.carUnloadPlacesMap[car.active_car_id] || [],
            entry_date_to: car.active_entry_date_to,
            entry_time_from: car.active_entry_time_from,
            entry_time_to: car.active_entry_time_to,
            isActive: car.status,
            // Логин владельца сервер отдаёт только администратору, поэтому карточка
            // рисует строку по факту наличия значения, а не по своей проверке роли.
            user_name: car.user_name || null,
        };
        this.showDetailsViewModal = true;
    },
  },
};

export const employeesRegistryPeriodDetails = {
  computed: {
    entityDetailsProps() {
      return {
        show: this.showDetailsModal, employee: this.detailsEmployee,
        allTables: [], currentUserId: this.ownershipInfo?.user_id || null,
        currentUserName: '', source: 'employeesview',
      };
    },
  },
  methods: {
    async onEntityPeriodChanged(result) {
        const id = this.detailsEmployee?.id;
        if (!id || result.entity_id !== this.detailsEmployee.activeEmployeeId) return;
        const previous = this.employeesData.find(employee => employee.id === id);
        const period = result.effective_period;
        this.detailsEmployee = { ...this.detailsEmployee, ...period, period_mode: result.period_mode,
            pass_time: [period.entry_time_from?.slice(0, 5), period.entry_time_to?.slice(0, 5)].filter(Boolean).join(' - ') };
        await this.fetchEmployees();
        if (!this.showDetailsModal || this.detailsEmployee?.id !== id) return;
        const fresh = refreshedPeriodRow(this.employeesData, id, previous);
        if (fresh) this.openEmployeeDetails(fresh);
    },

    openEmployeeDetails(employee) {
        // EmployeeDetailsModal читает snake_case (last_name, position, ...)
        // и поддерживает source=employeesview - заголовок \"Информация о сотруднике\"
        this.detailsEmployee = {
            id: employee.id,
            // id заявочной строки активной заявки; по нему карточка тянет статус
            // территории (current-status ключуется по employees.id, не по реестру).
            activeEmployeeId: employee.active_employee_id || null,
            // id самой заявки - для кнопки "Открыть заявку" (open-application).
            applicationId: employee.active_application_id || null,
            last_name: employee.last_name,
            first_name: employee.first_name,
            middle_name: employee.middle_name,
            position: employee.position,
            citizenshipName: employee.citizenship_name,
            passport_series_number: employee.passport_series_number,
            patent_number: employee.patent_number,
            other_permission: employee.other_permission,
            organization: employee.active_app_org_name || employee.organization_name,
            company: employee.active_app_company_name || employee.company_name,
            entry_date_to: employee.active_entry_date_to,
            pass_time: employee.active_pass_time,
            isActive: employee.status,
            // Логин владельца сервер отдаёт только администратору, поэтому карточка
            // рисует строку по факту наличия значения, а не по своей проверке роли.
            user_name: employee.user_name || null,
            pd_consent_at: employee.pd_consent_at || null,
            target_tables: []
        };
        this.showDetailsModal = true;
    },
  },
};

export const applicationPeriodDetails = {
  computed: {
    vehicleDetailsProps() {
      return {
        show: this.showVehicleModal, vehicle: this.selectedVehicle,
        allUnloadingPlaces: this.allUnloadingPlaces, allTables: this.allTables,
        licensePlateFormats: this.licensePlateFormats,
        currentUserId: this.currentUserId, currentUserName: this.currentUserName,
        showCarFeatures: true, source: 'application',
        canOverride: this.canOverrideBlacklist, canCancelOverride: this.canManageBlacklistOverride,
      };
    },
    employeeDetailsProps() {
      return {
        show: this.showEmployeeModal, employee: this.selectedEmployee,
        allTables: this.allTables, currentUserId: this.currentUserId,
        currentUserName: this.currentUserName, source: 'application',
        canOverride: this.canOverrideBlacklist, canCancelOverride: this.canManageBlacklistOverride,
      };
    },
  },
  methods: {
    async onEntityPeriodChanged(kind, result) {
        const key = kind === 'vehicle' ? 'selectedVehicle' : 'selectedEmployee';
        const id = this[key]?.id;
        const applicationId = this.applicationData.id;
        if (!id || result.entity_id !== id) return;
        const previous = (kind === 'vehicle' ? this.attachmentCars : this.attachmentEmployees).find(row => row.id === id);
        const period = result.effective_period;
        this[key] = { ...this[key], ...period, period_mode: result.period_mode };
        if (kind === 'employee') this[key].pass_time = [period.entry_time_from?.slice(0, 5), period.entry_time_to?.slice(0, 5)].filter(Boolean).join(' - ');
        await this.loadApplicationDetails(this.applicationData, { preserveSelection: true });
        if (this.applicationData.id !== applicationId) return;
        if (this.$refs.historyComponent?.loadHistory) this.$refs.historyComponent.loadHistory();
        this.$emit('application-changed', this.applicationData);
        if (this[key]?.id !== id || !(kind === 'vehicle' ? this.showVehicleModal : this.showEmployeeModal)) return;
        const fresh = refreshedPeriodRow(kind === 'vehicle' ? this.attachmentCars : this.attachmentEmployees, id, previous);
        if (fresh) {
            if (kind === 'vehicle') this.openVehicleModal(fresh);
            else this.openEmployeeModal(fresh);
        }
    },

    openVehicleModal(car) {
        this.selectedVehicle = {
            id: car.id,
            plateNumber: car.car_number,
            roof_access: car.roof_access,
            free_parking: car.free_parking,
            individual_roof_access: car.individual_roof_access,
            individual_free_parking: car.individual_free_parking,
            mark: car.car_brand,
            formatId: car.formatId || null,
            organization: car.organization || null,
            organizationId: car.organization_id || null,
            company: car.company || null,
            companyId: car.company_id || null,
            isExisting: true,
            unloadPlaces: car.unload_places ? car.unload_places.map(p => p.id) : [],
            target_tables: car.target_tables || [], // объектами: в них источник привязки (#2558)
            entry_date_to: car.entry_date_to || null,
            entry_time_from: car.entry_time_from || null,
            entry_time_to: car.entry_time_to || null,
            applicationId: this.applicationData.id,
            territory_status: 0,
            entry_checked: false,
            exit_checked: false,
            blacklist_similar: car.blacklist_similar || null
        };
        this.showVehicleModal = true;
    },

    openEmployeeModal(employee) {
        this.selectedEmployee = {
            id: employee.id,
            last_name: employee.last_name,
            first_name: employee.first_name,
            middle_name: employee.middle_name,
            position: employee.position,
            citizenshipName: employee.citizenship_name,
            passport_series_number: employee.passport_series_number,
            patent_number: employee.patent_number,
            other_permission: employee.other_permission,
            organization: employee.organization || null,
            organizationId: employee.organization_id || null,
            company: employee.company || null,
            companyId: employee.company_id || null,
            entry_date_to: employee.entry_date_to || null,
            pass_time: employee.pass_time || null,
            target_tables: employee.target_tables || [],
            applicationId: this.applicationData.id,
            territory_status: 0,
            blacklist_similar: employee.blacklist_similar || null
        };
        this.showEmployeeModal = true;
    },
  },
};
